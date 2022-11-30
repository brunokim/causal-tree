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
	loc := l.currLoc()
	xs, _, _ := l.tree.snapshotList(loc)
	return xs
}

func (l *List) Len() int {
	t := l.tree
	loc := l.currLoc()

	cnt := 0
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
			}
			j += size
		default:
			panic(fmt.Sprintf("list: unexpected tag: %v", atom.tag))
		}
	}
	return cnt
}

func (e *Elem) Clear() {
	loc := e.currLoc()

	j := loc + 1
	for j < len(e.tree.atoms) && e.tree.withinBlock(j, loc) {
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
			panic(fmt.Sprintf("elem @ %d: unexpected tag @ %d: %v", loc, j, atom.tag))
		}
	}
}

func (e *Elem) SetString() *String {
	loc := e.currLoc()
	return e.tree.newString(e.atomID, loc)
}

func (e *Elem) SetCounter() *Counter {
	loc := e.currLoc()
	return e.tree.newCounter(e.atomID, loc)
}

func (e *Elem) SetList() *List {
	loc := e.currLoc()
	return e.tree.newList(e.atomID, loc)
}

func (e *Elem) Value() Value {
	loc := e.currLoc()

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
			fmt.Println(e.tree.PrintTable())
			panic(fmt.Sprintf("elem @ %d: unexpected tag @ %d: %v", loc, j, atom.tag))
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
	loc := c.currLoc()

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

func (c *ListCursor) Element() *Elem {
	c.currLoc()
	return &Elem{c.treeLocation}
}

func (c *ListCursor) Insert() *Elem {
	loc := c.currLoc()
	id, charLoc := c.tree.addAtom(c.atomID, loc, elementTag, 0)
	c.atomID = id
	c.minLoc = charLoc
	return &Elem{c.treeLocation}
}

func (c *ListCursor) Delete() {
	loc := c.tree.deleteAtom(c.atomID, c.minLoc)
	c.atomID, c.minLoc = c.tree.findNonDeletedCause(loc)
}
