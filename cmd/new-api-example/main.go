package main

import (
	"fmt"
	"strings"
)

type Register interface {
	SetString() *String
	SetCounter() *Counter
	SetList() *List
	Value() Value
}

type Container interface {
	Cursor() Cursor
}

type Cursor interface {
	Index(i int)
	Value() Value
}

type Value interface {
	Delete()
}

// ----

func ValueAt(c Cursor, i int) Value {
	c.Index(i)
	return c.Value()
}

// ----

type AtomTag int

const (
	deleteTag AtomTag = iota
	charTag
	elementTag
	incrementTag
	stringTag
	counterTag
	listTag
)

func (tag AtomTag) String() string {
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
	return fmt.Sprintf("AtomTag(%d)", tag)
}

func priority(tag AtomTag) int {
	switch tag {
	case deleteTag:
		return -1
	case elementTag:
		return +1
	default:
		return 0
	}
}

type AtomID int32

type Atom struct {
	id      AtomID
	causeID AtomID

	tag   AtomTag
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

type CausalTree struct {
	atoms []Atom
}

func (t *CausalTree) PrintTable() string {
	var lastID AtomID = -1
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
func (t *CausalTree) searchAtom(id AtomID, minLoc int) int {
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
func (t *CausalTree) searchPositionFor(tag AtomTag, loc int) int {
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

func isExpectedTag(tag AtomTag, allowed ...AtomTag) bool {
	for _, other := range allowed {
		if tag == other {
			return true
		}
	}
	return false
}

func (t *CausalTree) validate(loc int, child AtomTag) bool {
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

func (t *CausalTree) addAtom(causeID AtomID, loc int, tag AtomTag, value int32) (AtomID, int) {
	if loc >= 0 && t.atoms[loc].id != causeID {
		panic(fmt.Errorf("cause loc-ID mismatch: loc=%d id=%d atoms[%d].id=%d", loc, causeID, loc, t.atoms[loc].id))
	}
	if !t.validate(loc, tag) {
		panic(fmt.Errorf("invalid child: loc=%d, tag=%v", loc, tag))
	}
	pos := t.searchPositionFor(tag, loc)
	id := AtomID(len(t.atoms) + 1)
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

func (t *CausalTree) deleteAtom(atomID AtomID, minLoc int) int {
	loc := t.searchAtom(atomID, minLoc)
	t.addAtom(atomID, loc, deleteTag, 0)
	return loc
}

func (t *CausalTree) findNonDeletedCause(loc int) (AtomID, int) {
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

func (t *CausalTree) newString(atomID AtomID, loc int) *String {
	id, loc := t.addAtom(atomID, loc, stringTag, 0)
	return &String{
		tree:   t,
		atomID: id,
		minLoc: loc,
	}
}

func (t *CausalTree) newCounter(atomID AtomID, loc int) *Counter {
	id, loc := t.addAtom(atomID, loc, counterTag, 0)
	return &Counter{
		tree:   t,
		atomID: id,
		minLoc: loc,
	}
}

func (t *CausalTree) newList(atomID AtomID, loc int) *List {
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

// ----

type String struct {
	tree   *CausalTree
	atomID AtomID
	minLoc int
}

type Char struct {
	tree   *CausalTree
	atomID AtomID
	minLoc int
}

func (s *String) StringCursor() *StringCursor {
	return &StringCursor{s.tree, s.atomID, s.minLoc}
}

func (s *String) Cursor() Cursor {
	return s.StringCursor()
}

func (s *String) Delete() {
	loc := s.tree.deleteAtom(s.atomID, s.minLoc)
	s.minLoc = loc
}

func (ch *Char) Delete() {
	loc := ch.tree.deleteAtom(ch.atomID, ch.minLoc)
	ch.minLoc = loc
}

// ----

type StringCursor struct {
	tree   *CausalTree
	atomID AtomID
	minLoc int
}

func (c *StringCursor) Index(i int) {
	if i < 0 {
		panic("Invalid negative index")
	}
	t := c.tree
	loc := t.searchAtom(c.atomID, c.minLoc)
	c.minLoc = loc

	cnt := -1
	j := loc + 1
	for j < len(t.atoms) && t.withinBlock(j, loc) {
		atom := t.atoms[j]
		switch atom.tag {
		case deleteTag:
			// String is already deleted, but do nothing
			j++
		case charTag:
			size, isDeleted := t.charBlock(j)
			if !isDeleted {
				cnt++
				if cnt == i {
					loc = j
					break
				}
			}
			j += size
		default:
			panic(fmt.Sprintf("string: unexpected tag: %v", atom.tag))
		}
	}
	if cnt < i {
		panic(fmt.Sprintf("string: index out of range: %d (size=%d)", i, cnt))
	}
	c.minLoc = loc
	c.atomID = t.atoms[loc].id
}

func (c *StringCursor) Char() *Char {
	loc := c.tree.searchAtom(c.atomID, c.minLoc)
	c.minLoc = loc
	return &Char{c.tree, c.atomID, loc}
}

func (c *StringCursor) Value() Value {
	return c.Char()
}

func (c *StringCursor) Insert(ch rune) (AtomID, int) {
	loc := c.tree.searchAtom(c.atomID, c.minLoc)
	id, charLoc := c.tree.addAtom(c.atomID, loc, charTag, int32(ch))
	c.atomID = id
	c.minLoc = charLoc
	return id, charLoc
}

func (c *StringCursor) Delete() {
	loc := c.tree.deleteAtom(c.atomID, c.minLoc)
	c.atomID, c.minLoc = c.tree.findNonDeletedCause(loc)
}

// ----

type Counter struct {
	tree   *CausalTree
	atomID AtomID
	minLoc int
}

func (cnt *Counter) increment(x int32) {
	loc := cnt.tree.searchAtom(cnt.atomID, cnt.minLoc)
	cnt.minLoc = loc
	cnt.tree.addAtom(cnt.atomID, loc, incrementTag, x)
}

func (cnt *Counter) Increment(x int32) { cnt.increment(+x) }
func (cnt *Counter) Decrement(x int32) { cnt.increment(-x) }

func (cnt *Counter) Delete() {
	loc := cnt.tree.deleteAtom(cnt.atomID, cnt.minLoc)
	cnt.minLoc = loc
}

// ----

type List struct {
	tree   *CausalTree
	atomID AtomID
	minLoc int
}

type Elem struct {
	tree   *CausalTree
	atomID AtomID
	minLoc int
}

func (l *List) ListCursor() *ListCursor {
	return &ListCursor{l.tree, l.atomID, l.minLoc}
}

func (l *List) Cursor() Cursor {
	return l.ListCursor()
}

func (l *List) Delete() {
	loc := l.tree.deleteAtom(l.atomID, l.minLoc)
	l.minLoc = loc
}

func (e *Elem) Delete() {
	loc := e.tree.deleteAtom(e.atomID, e.minLoc)
	e.minLoc = loc
}

func (e *Elem) SetString() *String {
	loc := e.tree.searchAtom(e.atomID, e.minLoc)
	e.minLoc = loc
	return e.tree.newString(e.atomID, loc)
}

func (e *Elem) SetCounter() *Counter {
	loc := e.tree.searchAtom(e.atomID, e.minLoc)
	e.minLoc = loc
	return e.tree.newCounter(e.atomID, loc)
}

func (e *Elem) SetList() *List {
	loc := e.tree.searchAtom(e.atomID, e.minLoc)
	e.minLoc = loc
	return e.tree.newList(e.atomID, loc)
}

func (e *Elem) Value() Value {
	loc := e.tree.searchAtom(e.atomID, e.minLoc)
	e.minLoc = loc

	j := loc + 1
	for j < len(e.tree.atoms) && e.tree.withinBlock(j, loc) {
		atom := e.tree.atoms[j]
		switch atom.tag {
		case deleteTag:
			// Elem is deleted, but do nothing.
			j++
		case stringTag, counterTag, listTag:
			return e.tree.valueOf(j)
		default:
			panic(fmt.Sprintf("elem: unexpected tag: %v", atom.tag))
		}
	}
	// Empty elem.
	return nil
}

// ----

type ListCursor struct {
	tree   *CausalTree
	atomID AtomID
	minLoc int
}

func (c *ListCursor) Index(i int) {
	if i < 0 {
		panic("Invalid negative index")
	}
	t := c.tree
	loc := t.searchAtom(c.atomID, c.minLoc)
	c.minLoc = loc

	cnt := -1
	j := loc + 1
	for j < len(t.atoms) && t.withinBlock(j, loc) {
		atom := t.atoms[j]
		switch atom.tag {
		case deleteTag:
			// List is already deleted, but do nothing
			j++
		case elementTag:
			size, isDeleted := t.elemBlock(j)
			if !isDeleted {
				cnt++
				if cnt == i {
					loc = j
					break
				}
			}
			j += size
		default:
			panic(fmt.Sprintf("list: unexpected tag: %v", atom.tag))
		}
	}
	if cnt < i {
		panic(fmt.Sprintf("list: index out of range: %d (size=%d)", i, cnt))
	}
	c.minLoc = loc
	c.atomID = t.atoms[loc].id
}

func (c *ListCursor) Elem() *Elem {
	loc := c.tree.searchAtom(c.atomID, c.minLoc)
	c.minLoc = loc
	return &Elem{c.tree, c.atomID, loc}
}

func (c *ListCursor) Value() Value {
	return c.Elem()
}

func (c *ListCursor) Insert() *Elem {
	loc := c.tree.searchAtom(c.atomID, c.minLoc)
	id, charLoc := c.tree.addAtom(c.atomID, loc, elementTag, 0)
	c.atomID = id
	c.minLoc = charLoc
	return &Elem{c.tree, id, charLoc}
}

func (c *ListCursor) Delete() {
	loc := c.tree.deleteAtom(c.atomID, c.minLoc)
	c.atomID, c.minLoc = c.tree.findNonDeletedCause(loc)
}

// ----

func main() {
	t := new(CausalTree)
	//
	{
		s1 := t.SetString()
		cursor := s1.StringCursor()
		cursor.Insert('c')
		cursor.Insert('r')
		cursor.Insert('d')
		cursor.Insert('t')
		fmt.Println("set string:", t.Snapshot())
	}
	//
	{
		s2 := t.SetString()
		cursor := s2.StringCursor()
		cursor.Insert('w')
		cursor.Insert('o')
		cursor.Insert('w')
		fmt.Println("set string:", t.Snapshot())
	}
	// Abstract walk, must know insertion order.
	{
		x1 := t.Value().(Container)
		x2 := ValueAt(x1.Cursor(), 2)
		x2.Delete()
		fmt.Println("delete $.2:", t.Snapshot())
	}
	// Insert a counter
	{
		cnt := t.SetCounter()
		cnt.Increment(45)
		cnt.Decrement(3)
		fmt.Println("set counter:", t.Snapshot())
	}
	// Insert list
	l1 := t.SetList()
	{
		cursor := l1.ListCursor()
		// counter element
		cursor.Insert().SetCounter().Increment(10)
		// nil element
		cursor.Insert()
		// string element
		strCursor := cursor.Insert().SetString().StringCursor()
		strCursor.Insert('d')
		strCursor.Insert('o')
		strCursor.Insert('g')

		fmt.Println("set list:", t.Snapshot())
	}
	// Modify embedded counter
	{
		cnt := ValueAt(l1.Cursor(), 0).(*Elem).Value().(*Counter)
		cnt.Increment(40)
		cnt.Decrement(8)
		fmt.Println("modify counter:", t.Snapshot())
	}
	// Modify a string after it was pushed to the right by the previous insertion.
	{
		s2 := ValueAt(l1.Cursor(), 2).(*Elem).Value().(*String)
		cursor := s2.StringCursor()
		cursor.Index(1)
		cursor.Delete()
		cursor.Delete()
		cursor.Insert('f')
		cursor.Insert('i')
		fmt.Println("modify string:", t.Snapshot())
	}
	// Delete elem
	{
		cursor := l1.ListCursor()
		cursor.Index(1)
		cursor.Delete()
		fmt.Println("delete elem:", t.Snapshot())
	}
	// Delete counter and mutate after deletion.
	{
		cnt := ValueAt(l1.Cursor(), 0).(*Elem).Value().(*Counter)
		cnt.Delete()
		cnt.Increment(27)
		fmt.Println("delete counter:", t.Snapshot())
	}
	// Insert char having deleted character as parent.
	{
		s1 := ValueAt(l1.Cursor(), 1).(*Elem).Value().(*String)
		c1 := s1.StringCursor()
		c1.Index(2)
		c1.Insert('-')
		c1.Insert('z')
		c1.Insert('y')

		c2 := s1.StringCursor()
		c2.Index(5)
		c2.Delete()

		c1.Insert('x')
		c2.Insert('w')
		fmt.Println("modify string:", t.Snapshot())
	}

	fmt.Println(t.PrintTable())
}
