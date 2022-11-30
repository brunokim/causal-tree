package crdt

import (
	"fmt"
)

// String contains a mutable persistent string, or a sequence of Unicode codepoints ("chars").
type String struct {
	treePosition
}

func (*String) isValue() {}

// Char represents a Unicode codepoint within a String.
type Char struct {
	treePosition
}

func (s *String) Snapshot() string {
	pos := s.currPos()
	str, _, _ := s.tree.snapshotString(pos)
	return str
}

func (s *String) StringCursor() *StringCursor {
	return &StringCursor{s.treePosition}
}

func (s *String) Cursor() Cursor {
	return s.StringCursor()
}

func (l *String) Len() int {
	t := l.tree
	pos := l.currPos()

	cnt := 0
	j := pos + 1
	for j < len(t.atoms) && t.withinBlock(j, pos) {
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
	treePosition
}

func (c *StringCursor) Index(i int) {
	if i < -1 {
		panic("Invalid index")
	}
	t := c.tree
	pos := c.currPos()

	cnt := -1
	j := pos + 1
	for j < len(t.atoms) && t.withinBlock(j, pos) {
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
					pos = j
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
	c.lastKnownPos = pos
	c.atomID = t.atoms[pos].id
}

func (c *StringCursor) Element() *Char {
	c.currPos()
	return &Char{c.treePosition}
}

func (c *StringCursor) Insert(ch rune) *Char {
	pos := c.currPos()
	id, charLoc := c.tree.addAtom(c.atomID, pos, charTag, int32(ch))
	c.atomID = id
	c.lastKnownPos = charLoc
	return &Char{c.treePosition}
}

func (c *StringCursor) Delete() {
	pos := c.tree.deleteAtom(c.atomID, c.lastKnownPos)
	c.atomID, c.lastKnownPos = c.tree.findNonDeletedCause(pos)
}
