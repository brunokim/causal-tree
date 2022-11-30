// Exercising a new API for the main Causal Tree implementation.
package crdt

import (
	"fmt"
	"strings"
)

// Register contains a single value or none at all.
type Register interface {
	// SetString sets the register to an empty string.
	SetString() *String
	// SetString sets the register to a zeroed counter.
	SetCounter() *Counter
	// SetString sets the register to an empty list.
	SetList() *List
	// Clear resets the register to an empty state.
	Clear()
	// Value returns the underlying value.
	Value() Value
}

// Container represents a collection of values.
type Container interface {
	// Len walks the container and returns the number of elements.
	Len() int
	// Cursor returns the container's cursor initialized to its head position (index=-1).
	Cursor() Cursor
}

// Cursor represents a pointer to a container's element, or the container's head.
// Concrete cursors have additional methods with type-specific parameters and return type:
//
// - Insert() inserts a new element after the current position.
//
// - Element() returns the pointed element. It panics if the cursor is pointing to the container's head.
type Cursor interface {
	// Index moves the cursor to the i-th element (container's head=-1). It panics if i is out of bounds.
	Index(i int)
	// Delete removes the pointed element from the collection. The cursor is moved to the
	// previous element, or the container's head. It panics if the cursor is pointing to the container's
	// head.
	Delete()
}

// Value represents a structure that may be converted to concrete data.
// Concrete Values have a Snapshot() method that return the value's Go representation.
type Value interface {
	isValue()
}

// ----

// Snapshot returns the Value's Go representation.
func Snapshot(value Value) interface{} {
	switch v := value.(type) {
	case *String:
		return v.Snapshot()
	case *Counter:
		return v.Snapshot()
	case *List:
		return v.Snapshot()
	default:
		panic(fmt.Sprintf("unknown value %T", value))
	}
}

// Element returns the Cursor's pointed element.
func Element(cursor Cursor) interface{} {
	switch c := cursor.(type) {
	case *StringCursor:
		return c.Element()
	case *ListCursor:
		return c.Element()
	default:
		panic(fmt.Sprintf("unknown cursor %T", cursor))
	}
}

// ElementAt moves the cursor to the i-th position and returns the element there.
func ElementAt(c Cursor, i int) interface{} {
	c.Index(i)
	return Element(c)
}

// ----

type atomTag int

const (
	deleteTag atomTag = iota
	charTag
	elementTag
	incrementTag
	stringTag
	counterTag
	listTag
)

func (tag atomTag) String() string {
	switch tag {
	case deleteTag:
		return "delete"
	case charTag:
		return "char"
	case elementTag:
		return "element"
	case incrementTag:
		return "increment"
	case stringTag:
		return "string"
	case counterTag:
		return "counter"
	case listTag:
		return "list"
	}
	return fmt.Sprintf("atomTag(%d)", tag)
}

func priority(tag atomTag) int {
	switch tag {
	case deleteTag:
		return -1
	case elementTag:
		return +1
	default:
		return 0
	}
}

type atomID int32

type Atom struct {
	id      atomID
	causeID atomID

	tag   atomTag
	value int32 // rune for Char, increment for Increment
}

func (a Atom) printValue() string {
	switch a.tag {
	case deleteTag:
		return "\u232b"
	case charTag:
		return fmt.Sprintf("char %c", a.value)
	case elementTag:
		return "element"
	case incrementTag:
		return fmt.Sprintf("inc %d", a.value)
	case stringTag:
		return "string"
	case counterTag:
		return "counter"
	case listTag:
		return "list"
	}
	return "(unknown)"
}

// CausalTree is a Register and a Value containing other data structures.
type CausalTree struct {
	atoms []Atom
}

func (*CausalTree) isValue() {}

// PrintTable prints the internal tree's atom structure for debugging.
func (t *CausalTree) PrintTable() string {
	var lastID atomID = -1
	var sb strings.Builder
	fmt.Fprintf(&sb, " cause |   id |      value \n")
	fmt.Fprintf(&sb, "-------|------|------------\n")
	for _, atom := range t.atoms {
		if atom.causeID != lastID {
			fmt.Fprintf(&sb, "  %04d | %04d | %10s\n", atom.causeID, atom.id, atom.printValue())
		} else {
			fmt.Fprintf(&sb, "       | %04d | %10s\n", atom.id, atom.printValue())
		}
		lastID = atom.id
	}
	return sb.String()
}

// ----

// treePosition represents a given atom's position in a causal tree.
type treePosition struct {
	tree   *CausalTree
	atomID atomID

	// treePosition stores the last known position within the atoms slice to speed-up searching:
	// since atoms can only be inserted, its actual position may only be at or to the right
	// of the latest known position.
	lastKnownPos int
}

func (c *treePosition) currPos() int {
	pos := c.tree.searchAtom(c.atomID, c.lastKnownPos)
	c.lastKnownPos = pos
	return pos
}

// ----

// Search for an atom given by 'id' and last seen at 'lastKnownPos'
func (t *CausalTree) searchAtom(id atomID, lastKnownPos int) int {
	if id == 0 {
		return -1
	}
	for i := lastKnownPos; i < len(t.atoms); i++ {
		if t.atoms[i].id == id {
			return i
		}
	}
	return -1
}

// Search for an insertion position for the given 'tag', with parent at 'pos'.
func (t *CausalTree) searchIndexFor(tag atomTag, pos int) int {
	if pos < 0 {
		return 0
	}
	for i := pos + 1; i < len(t.atoms); i++ {
		if t.atoms[i].causeID < t.atoms[pos].id {
			// End of causal block.
			return i
		}
		if t.atoms[i].causeID == t.atoms[pos].id {
			// Sibling atom, sort by tag.
			if priority(t.atoms[i].tag) >= priority(tag) {
				return i
			}
		}
	}
	return len(t.atoms)
}

func isExpectedTag(tag atomTag, allowed ...atomTag) bool {
	for _, other := range allowed {
		if tag == other {
			return true
		}
	}
	return false
}

func (t *CausalTree) validate(pos int, child atomTag) bool {
	if pos < 0 {
		return isExpectedTag(child, stringTag, counterTag, listTag)
	}
	parent := t.atoms[pos].tag
	switch parent {
	case deleteTag:
		return false
	case charTag:
		return isExpectedTag(child, deleteTag, charTag)
	case elementTag:
		return isExpectedTag(child, deleteTag, elementTag, stringTag, counterTag, listTag)
	case incrementTag:
		return false
	case stringTag:
		return isExpectedTag(child, deleteTag, charTag)
	case counterTag:
		return isExpectedTag(child, deleteTag, incrementTag)
	case listTag:
		return isExpectedTag(child, deleteTag, elementTag)
	default:
		panic(fmt.Sprintf("unexpected tag %v", parent))
	}
}

func (t *CausalTree) addAtom(causeID atomID, pos int, tag atomTag, value int32) (atomID, int) {
	if pos >= 0 && t.atoms[pos].id != causeID {
		panic(fmt.Errorf("cause pos-ID mismatch: pos=%d id=%d atoms[%d].id=%d", pos, causeID, pos, t.atoms[pos].id))
	}
	if !t.validate(pos, tag) {
		panic(fmt.Errorf("invalid child: pos=%d, tag=%v", pos, tag))
	}
	i := t.searchIndexFor(tag, pos)
	id := atomID(len(t.atoms) + 1)
	t.atoms = append(t.atoms, Atom{})
	copy(t.atoms[i+1:], t.atoms[i:])
	t.atoms[i] = Atom{id, causeID, tag, value}

	return id, i
}

func (t *CausalTree) withinBlock(j, i int) bool {
	return t.atoms[j].causeID >= t.atoms[i].id
}

// Returns the size of a single char block.
func (t *CausalTree) charBlock(i int) (int, bool) {
	j := i + 1
	isDeleted := false
loop:
	for j < len(t.atoms) && t.withinBlock(j, i) {
		atom := t.atoms[j]
		switch atom.tag {
		case deleteTag:
			isDeleted = true
			j++
		case charTag:
			break loop
		default:
			fmt.Println(t.PrintTable())
			panic(fmt.Sprintf("char @ %d: unexpected tag @ %d: %v", i, j, atom.tag))
		}
	}
	return j - i, isDeleted
}

// Returns the size of a single elem block.
func (t *CausalTree) elemBlock(i int) (int, bool) {
	j := i + 1
	isDeleted := false
loop:
	for j < len(t.atoms) && t.withinBlock(j, i) {
		atom := t.atoms[j]
		switch atom.tag {
		case deleteTag:
			isDeleted = true
			j++
		case stringTag, counterTag, listTag:
			size := t.causalBlockSize(j)
			j += size
			break loop
		case elementTag:
			break loop
		default:
			fmt.Println(t.PrintTable())
			panic(fmt.Sprintf("elem @ %d: unexpected tag @ %d: %v", i, j, atom.tag))
		}
	}
	return j - i, isDeleted
}

// Returns the size of a causal block.
func (t *CausalTree) causalBlockSize(i int) int {
	j := i + 1
	for j < len(t.atoms) {
		if t.atoms[j].causeID < t.atoms[i].id {
			break
		}
		j++
	}
	return j - i
}

func (t *CausalTree) deleteAtom(atomID atomID, lastKnownPos int) int {
	pos := t.searchAtom(atomID, lastKnownPos)
	t.addAtom(atomID, pos, deleteTag, 0)
	return pos
}

func (t *CausalTree) findNonDeletedCause(pos int) (atomID, int) {
	causeID := t.atoms[pos].causeID
	isDeleted := false
	for i := pos - 1; i >= 0; i-- {
		if causeID == 0 {
			// Cause is the root atom.
			pos = -1
			break
		}
		atom := t.atoms[i]
		if atom.tag == deleteTag && atom.causeID == causeID {
			// Cause is also deleted.
			isDeleted = true
			continue
		}
		if atom.id != causeID {
			continue
		}
		if isDeleted {
			// Found cause, which is deleted. Reset cause to its parent, and keep searching.
			causeID = atom.causeID
			isDeleted = false
			continue
		}
		// Found existing cause, set its location.
		pos = i
		break
	}
	return causeID, pos
}

// ----

func (t *CausalTree) newValue(atomID atomID, pos int, tag atomTag) treePosition {
	id, pos := t.addAtom(atomID, pos, tag, 0)
	return treePosition{
		tree:         t,
		atomID:       id,
		lastKnownPos: pos,
	}
}

func (t *CausalTree) newString(atomID atomID, pos int) *String {
	return &String{t.newValue(atomID, pos, stringTag)}
}

func (t *CausalTree) newCounter(atomID atomID, pos int) *Counter {
	return &Counter{t.newValue(atomID, pos, counterTag)}
}

func (t *CausalTree) newList(atomID atomID, pos int) *List {
	return &List{t.newValue(atomID, pos, listTag)}
}

func (t *CausalTree) SetString() *String   { return t.newString(0, -1) }
func (t *CausalTree) SetCounter() *Counter { return t.newCounter(0, -1) }
func (t *CausalTree) SetList() *List       { return t.newList(0, -1) }
func (t *CausalTree) Value() Value         { return t.valueOf(0) }
func (t *CausalTree) Clear()               { t.deleteAtom(0, -1) }

func (t *CausalTree) valueOf(i int) Value {
	atom := t.atoms[i]
	switch atom.tag {
	case stringTag:
		return &String{treePosition{t, atom.id, i}}
	case counterTag:
		return &Counter{treePosition{t, atom.id, i}}
	case listTag:
		return &List{treePosition{t, atom.id, i}}
	default:
		panic(fmt.Sprintf("valueOf: unexpected tag: %v", atom.tag))
	}
}

// ----

func (t *CausalTree) Snapshot() interface{} {
	obj, _, isDeleted := t.snapshot(0)
	if isDeleted {
		return nil
	}
	return obj
}

func (t *CausalTree) snapshot(i int) (interface{}, int, bool) {
	atom := t.atoms[i]
	switch atom.tag {
	case stringTag:
		return t.snapshotString(i)
	case counterTag:
		return t.snapshotCounter(i)
	case listTag:
		return t.snapshotList(i)
	default:
		fmt.Println(t.PrintTable())
		panic(fmt.Sprintf("snapshot @ %d: unexpected tag: %v", i, atom.tag))
	}
}

func (t *CausalTree) snapshotString(i int) (string, int, bool) {
	var sb strings.Builder
	j := i + 1
	isDeleted := false
	for j < len(t.atoms) && t.withinBlock(j, i) {
		atom := t.atoms[j]
		switch atom.tag {
		case deleteTag:
			isDeleted = true
			j++
		case charTag:
			ch, size, charDeleted := t.snapshotChar(j)
			if !charDeleted {
				sb.WriteRune(ch)
			}
			j += size
		default:
			fmt.Println(t.PrintTable())
			panic(fmt.Sprintf("string @ %d: unexpected tag @ %d: %v", i, j, atom.tag))
		}
	}
	return sb.String(), j - i, isDeleted
}

func (t *CausalTree) snapshotChar(i int) (rune, int, bool) {
	size, isDeleted := t.charBlock(i)
	return rune(t.atoms[i].value), size, isDeleted
}

func (t *CausalTree) snapshotCounter(i int) (int32, int, bool) {
	var sum int32
	j := i + 1
	isDeleted := false
	for j < len(t.atoms) && t.withinBlock(j, i) {
		atom := t.atoms[j]
		switch atom.tag {
		case deleteTag:
			isDeleted = true
			j++
		case incrementTag:
			sum += atom.value
			j++
		default:
			fmt.Println(t.PrintTable())
			panic(fmt.Sprintf("counter @ %d: unexpected tag @ %d: %v", i, j, atom.tag))
		}
	}
	return sum, j - i, isDeleted
}

func (t *CausalTree) snapshotList(i int) ([]interface{}, int, bool) {
	var result []interface{}
	j := i + 1
	isDeleted := false
	for j < len(t.atoms) && t.withinBlock(j, i) {
		atom := t.atoms[j]
		switch atom.tag {
		case deleteTag:
			isDeleted = true
			j++
		case elementTag:
			elem, size, elemDeleted := t.snapshotElem(j)
			if !elemDeleted {
				result = append(result, elem)
			}
			j += size
		default:
			fmt.Println(t.PrintTable())
			panic(fmt.Sprintf("list @ %d: unexpected tag @ %d: %v", i, j, atom.tag))
		}
	}
	return result, j - i, isDeleted
}

func (t *CausalTree) snapshotElem(i int) (interface{}, int, bool) {
	var value interface{}
	j := i + 1
	isDeleted := false
loop:
	for j < len(t.atoms) && t.withinBlock(j, i) {
		atom := t.atoms[j]
		switch atom.tag {
		case deleteTag:
			isDeleted = true
			j++
		case stringTag, counterTag, listTag:
			elem, size, elemDeleted := t.snapshot(j)
			if !elemDeleted {
				value = elem
			}
			j += size
			break loop
		case elementTag:
			break loop
		default:
			fmt.Println(t.PrintTable())
			panic(fmt.Sprintf("elem @ %d: unexpected tag @ %d: %v", i, j, atom.tag))
		}
	}
	return value, j - i, isDeleted
}
