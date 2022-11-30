package crdt

import (
	"fmt"
)

// List is a Container of arbitrary elements in a specific order.
type List struct {
	treeLocation
}

func (*List) isValue() {}

// Elem is a list's element representation as a register, that may contain any other values.
type Elem struct {
	treeLocation
}

func (l *List) ListCursor() *ListCursor {
	return &ListCursor{l.treeLocation}
}

func (l *List) Cursor() Cursor {
	return l.ListCursor()
}

func (l *List) Snapshot() []interface{} {
	pos := l.currPos()
	xs, _, _ := l.tree.snapshotList(pos)
	return xs
}

func (l *List) Len() int {
	t := l.tree
	pos := l.currPos()

	cnt := 0
	j := pos + 1
	for j < len(t.atoms) && t.withinBlock(j, pos) {
		atom := t.atoms[j]
		switch atom.tag {
		case deleteTag:
			// List is already deleted, but do nothing
			j++
		case elementTag:
			size, isDeleted := t.elemBlock(j)
			if !isDeleted {
				cnt++
			}
			j += size
		default:
			panic(fmt.Sprintf("list: unexpected tag: %v", atom.tag))
		}
	}
	return cnt
}

func (e *Elem) Clear() {
	pos := e.currPos()

	j := pos + 1
	for j < len(e.tree.atoms) && e.tree.withinBlock(j, pos) {
		atom := e.tree.atoms[j]
		switch atom.tag {
		case deleteTag:
			// Elem is deleted, but do nothing.
			j++
		case stringTag, counterTag, listTag:
			// Delete contents of elem.
			e.tree.deleteAtom(atom.id, j)
			return
		default:
			fmt.Println(e.tree.PrintTable())
			panic(fmt.Sprintf("elem @ %d: unexpected tag @ %d: %v", pos, j, atom.tag))
		}
	}
}

func (e *Elem) SetString() *String {
	pos := e.currPos()
	return e.tree.newString(e.atomID, pos)
}

func (e *Elem) SetCounter() *Counter {
	pos := e.currPos()
	return e.tree.newCounter(e.atomID, pos)
}

func (e *Elem) SetList() *List {
	pos := e.currPos()
	return e.tree.newList(e.atomID, pos)
}

func (e *Elem) Value() Value {
	pos := e.currPos()

	j := pos + 1
	for j < len(e.tree.atoms) && e.tree.withinBlock(j, pos) {
		atom := e.tree.atoms[j]
		switch atom.tag {
		case deleteTag:
			// Elem is deleted, but do nothing.
			j++
		case stringTag, counterTag, listTag:
			return e.tree.valueOf(j)
		default:
			fmt.Println(e.tree.PrintTable())
			panic(fmt.Sprintf("elem @ %d: unexpected tag @ %d: %v", pos, j, atom.tag))
		}
	}
	// Empty elem.
	return nil
}

// ----

// ListCursor is a Cursor that walks and modifies a List.
type ListCursor struct {
	treeLocation
}

func (c *ListCursor) Index(i int) {
	if i < -1 {
		panic("Invalid negative index")
	}
	t := c.tree
	pos := c.currPos()

	cnt := -1
	j := pos + 1
	for j < len(t.atoms) && t.withinBlock(j, pos) {
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
					pos = j
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
	c.lastKnownPos = pos
	c.atomID = t.atoms[pos].id
}

func (c *ListCursor) Element() *Elem {
	c.currPos()
	return &Elem{c.treeLocation}
}

func (c *ListCursor) Insert() *Elem {
	pos := c.currPos()
	id, charLoc := c.tree.addAtom(c.atomID, pos, elementTag, 0)
	c.atomID = id
	c.lastKnownPos = charLoc
	return &Elem{c.treeLocation}
}

func (c *ListCursor) Delete() {
	pos := c.tree.deleteAtom(c.atomID, c.lastKnownPos)
	c.atomID, c.lastKnownPos = c.tree.findNonDeletedCause(pos)
}
