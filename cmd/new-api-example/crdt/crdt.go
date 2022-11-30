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

// Search for an atom given by 'id' and last seen at 'minLoc'
func (t *CausalTree) searchAtom(id atomID, minLoc int) int {
	if id == 0 {
		return -1
	}
	for i := minLoc; i < len(t.atoms); i++ {
		if t.atoms[i].id == id {
			return i
		}
	}
	return -1
}

// Search for an insertion position for the given 'tag', with parent at 'loc'.
func (t *CausalTree) searchPositionFor(tag atomTag, loc int) int {
	if loc < 0 {
		return 0
	}
	for i := loc + 1; i < len(t.atoms); i++ {
		if t.atoms[i].causeID < t.atoms[loc].id {
			// End of causal block.
			return i
		}
		if t.atoms[i].causeID == t.atoms[loc].id {
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

func (t *CausalTree) validate(loc int, child atomTag) bool {
	if loc < 0 {
		return isExpectedTag(child, stringTag, counterTag, listTag)
	}
	parent := t.atoms[loc].tag
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

func (t *CausalTree) addAtom(causeID atomID, loc int, tag atomTag, value int32) (atomID, int) {
	if loc >= 0 && t.atoms[loc].id != causeID {
		panic(fmt.Errorf("cause loc-ID mismatch: loc=%d id=%d atoms[%d].id=%d", loc, causeID, loc, t.atoms[loc].id))
	}
	if !t.validate(loc, tag) {
		panic(fmt.Errorf("invalid child: loc=%d, tag=%v", loc, tag))
	}
	pos := t.searchPositionFor(tag, loc)
	id := atomID(len(t.atoms) + 1)
	t.atoms = append(t.atoms, Atom{})
	copy(t.atoms[pos+1:], t.atoms[pos:])
	t.atoms[pos] = Atom{id, causeID, tag, value}

	return id, pos
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

func (t *CausalTree) deleteAtom(atomID atomID, minLoc int) int {
	loc := t.searchAtom(atomID, minLoc)
	t.addAtom(atomID, loc, deleteTag, 0)
	return loc
}

func (t *CausalTree) findNonDeletedCause(loc int) (atomID, int) {
	causeID := t.atoms[loc].causeID
	isDeleted := false
	for i := loc - 1; i >= 0; i-- {
		if causeID == 0 {
			// Cause is the root atom.
			loc = -1
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
		loc = i
		break
	}
	return causeID, loc
}

// ----

func (t *CausalTree) newString(atomID atomID, loc int) *String {
	id, loc := t.addAtom(atomID, loc, stringTag, 0)
	return &String{
		tree:   t,
		atomID: id,
		minLoc: loc,
	}
}

func (t *CausalTree) newCounter(atomID atomID, loc int) *Counter {
	id, loc := t.addAtom(atomID, loc, counterTag, 0)
	return &Counter{
		tree:   t,
		atomID: id,
		minLoc: loc,
	}
}

func (t *CausalTree) newList(atomID atomID, loc int) *List {
	id, loc := t.addAtom(atomID, loc, listTag, 0)
	return &List{
		tree:   t,
		atomID: id,
		minLoc: loc,
	}
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
		return &String{t, atom.id, i}
	case counterTag:
		return &Counter{t, atom.id, i}
	case listTag:
		return &List{t, atom.id, i}
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
