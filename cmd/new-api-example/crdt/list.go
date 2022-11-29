package crdt

import (
	"fmt"
)

// List is a Container of arbitrary elements in a specific order.
type List struct {
	tree   *CausalTree
	atomID atomID
	minLoc int
}

func (*List) isValue() {}

// Elem is a list's element representation as a register, that may contain any other values.
type Elem struct {
	tree   *CausalTree
	atomID atomID
	minLoc int
}

func (l *List) ListCursor() *ListCursor {
	return &ListCursor{l.tree, l.atomID, l.minLoc}
}

func (l *List) Cursor() Cursor {
	return l.ListCursor()
}

func (l *List) Snapshot() []interface{} {
	loc := l.tree.searchAtom(l.atomID, l.minLoc)
	xs, _, _ := l.tree.snapshotList(loc)
	return xs
}

func (e *Elem) Clear() {
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
	tree   *CausalTree
	atomID atomID
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

func (c *ListCursor) Element() *Elem {
	loc := c.tree.searchAtom(c.atomID, c.minLoc)
	c.minLoc = loc
	return &Elem{c.tree, c.atomID, loc}
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
