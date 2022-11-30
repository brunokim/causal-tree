package crdt

import (
	"fmt"
)

// String contains a mutable persistent string, or a sequence of Unicode codepoints ("chars").
type String struct {
	treeLocation
}

func (*String) isValue() {}

// Char represents a Unicode codepoint within a String.
type Char struct {
	treeLocation
}

func (s *String) Snapshot() string {
	loc := s.currLoc()
	str, _, _ := s.tree.snapshotString(loc)
	return str
}

func (s *String) StringCursor() *StringCursor {
	return &StringCursor{s.treeLocation}
}

func (s *String) Cursor() Cursor {
	return s.StringCursor()
}

func (l *String) Len() int {
	t := l.tree
	loc := l.currLoc()

	cnt := 0
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
			}
			j += size
		default:
			panic(fmt.Sprintf("string: unexpected tag: %v", atom.tag))
		}
	}
	return cnt
}

// ----

// StringCursor is a Cursor for walking and modifying a string.
type StringCursor struct {
	treeLocation
}

func (c *StringCursor) Index(i int) {
	if i < -1 {
		panic("Invalid index")
	}
	t := c.tree
	loc := c.currLoc()

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

func (c *StringCursor) Element() *Char {
	c.currLoc()
	return &Char{c.treeLocation}
}

func (c *StringCursor) Insert(ch rune) *Char {
	loc := c.currLoc()
	id, charLoc := c.tree.addAtom(c.atomID, loc, charTag, int32(ch))
	c.atomID = id
	c.minLoc = charLoc
	return &Char{c.treeLocation}
}

func (c *StringCursor) Delete() {
	loc := c.tree.deleteAtom(c.atomID, c.minLoc)
	c.atomID, c.minLoc = c.tree.findNonDeletedCause(loc)
}
