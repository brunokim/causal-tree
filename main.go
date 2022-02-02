package main

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"unicode"

	"github.com/google/uuid"
)

// Based on http://archagon.net/blog/2018/03/24/data-laced-with-history/

// Atom represents an atomic operation within a replicated list.
type Atom struct {
	// ID is the identifier of this atom.
	ID AtomID
	// Cause is the identifier of the preceding atom.
	Cause AtomID
	// Value is the data operation represented by this atom.
	Value AtomValue
}

// AtomID is the unique identifier of an atom.
type AtomID struct {
	// Site is index in the sitemap of the site that created an atom.
	Site uint16
	// Index is the order of creation of this atom in the given site.
	Index uint32
	// Timestamp is the Lamport timestamp of the site when the atom was created.
	Timestamp uint32
}

// AtomValue is a list operation.
type AtomValue interface {
	isAtomValue()
}

// InsertChar represents insertion of a char to the right of another atom.
type InsertChar struct {
	// Char inserted in list.
	Char rune
}

// Delete represents deleting an element from the list.
type Delete struct{}

func (v InsertChar) isAtomValue() {}
func (v Delete) isAtomValue()     {}

// RList is a replicated list data structure.
type RList struct {
	// Weave is the flat representation of a causal tree.
	Weave []Atom
	// Cursor is the index in the weave of the latest inserted element.
	Cursor uint32
	// Yarns is the list of atoms, grouped by the site that created them.
	Yarns [][]Atom
	// Sitemap is the ordered list of site IDs. The index in this sitemap is used to represent a site in atoms
	// and yarns.
	Sitemap []uuid.UUID
	// SiteID is this list's site UUIDv1.
	SiteID uuid.UUID
	// Timestamp is this list's Lamport timestamp.
	Timestamp uint32
}

// Create UUIDv1, using local timestamp as lower bits.
func uuidv1() uuid.UUID {
	id, err := uuid.NewUUID()
	if err != nil {
		panic(fmt.Sprintf("creating UUIDv1: %v", err))
	}
	return id
}

// NewRList creates an initialized empty replicated list.
func NewRList() *RList {
	siteID := uuidv1()
	return &RList{
		Weave:     nil,
		Cursor:    0,
		Yarns:     [][]Atom{nil},
		Sitemap:   []uuid.UUID{siteID},
		SiteID:    siteID,
		Timestamp: 1,
	}
}

// Returns the index of a site is (or should be) in the sitemap.
func (l *RList) siteIndex(siteID uuid.UUID) int {
	return sort.Search(len(l.Sitemap), func(i int) bool {
		return bytes.Compare(l.Sitemap[i][:], siteID[:]) >= 0
	})
}

// Returns the index of an atom within the weave.
// NOTE: current implementation is very naive in O(n), should look for better algo.
func (l *RList) atomIndex(atomID AtomID) int {
	for i, atom := range l.Weave {
		if atom.ID == atomID {
			return i
		}
	}
	return len(l.Weave)
}

func (l *RList) insertAtom(atom Atom) {
	i := l.Cursor
	l.Weave = append(l.Weave, Atom{})
	copy(l.Weave[i+1:], l.Weave[i:])
	l.Weave[i] = atom
}

// Fork a replicated list into an independent object.
func (l *RList) Fork() *RList {
	if len(l.Sitemap)-1 >= math.MaxUint16 {
		panic("fork: reached limit of sites")
	}
	newSiteID := uuidv1()
	n := len(l.Sitemap)
	i := l.siteIndex(newSiteID)
	if i < n {
		panic("Not implemented yet: move yarns and renumber atoms")
	} else {
		l.Yarns = append(l.Yarns, nil)
		l.Sitemap = append(l.Sitemap, newSiteID)
	}
	l.Timestamp++
	ll := &RList{
		Weave:     make([]Atom, len(l.Weave)),
		Cursor:    l.Cursor,
		Yarns:     make([][]Atom, n+1),
		Sitemap:   make([]uuid.UUID, n+1),
		SiteID:    newSiteID,
		Timestamp: l.Timestamp,
	}
	copy(ll.Weave, l.Weave)
	for i, yarn := range l.Yarns {
		ll.Yarns[i] = make([]Atom, len(yarn))
		copy(ll.Yarns[i], yarn)
	}
	copy(ll.Sitemap, l.Sitemap)
	return ll
}

func (l *RList) addAtom(value AtomValue) {
	l.Timestamp++
	if l.Timestamp == 0 {
		// Overflow
		panic("appending atom: reached limit of states")
	}
	i := l.siteIndex(l.SiteID)
	atomID := AtomID{
		Site:      uint16(i),
		Index:     uint32(len(l.Yarns[i])),
		Timestamp: l.Timestamp,
	}
	var cause AtomID
	if l.Cursor > 0 {
		cause = l.Weave[l.Cursor-1].ID
	}
	atom := Atom{
		ID:    atomID,
		Cause: cause,
		Value: value,
	}
	l.insertAtom(atom)
	l.Yarns[i] = append(l.Yarns[i], atom)
}

// InsertCharAfter inserts a char after the cursor position.
func (l *RList) InsertChar(ch rune) {
	l.addAtom(InsertChar{ch})
	l.Cursor++
}

// DeleteChar deletes the char before the cursor position.
func (l *RList) DeleteChar() {
	if l.Cursor == 0 {
		panic("delete char: no atom to delete")
	}
	l.addAtom(Delete{})
	deletedAtom := l.Weave[l.Cursor-1]
	prevID := deletedAtom.Cause
	l.Cursor = uint32(l.atomIndex(prevID) + 1)
}

// AsString interprets list as a sequence of chars.
func (l *RList) AsString() string {
	// Fill in chars with runes from weave. Deleted chars are represented with an invalid rune.
	chars := make([]rune, len(l.Weave))
    hasDeleted := false
	for i, atom := range l.Weave {
		switch v := atom.Value.(type) {
		case InsertChar:
			chars[i] = v.Char
		case Delete:
            hasDeleted = true
			j := l.atomIndex(atom.Cause)
			chars[i] = unicode.MaxRune + 1
			chars[j] = unicode.MaxRune + 1
        default:
            panic(fmt.Sprintf("AsString: unexpected atom value type %T (%v)", atom.Value, atom.Value))
		}
	}
    if !hasDeleted {
        // Cheap optimization for case where there are no deletions.
        return string(chars)
    }
	// Move chars to fill in holes of invalid runes.
	deleted := 0
	for i, ch := range chars {
		if ch > unicode.MaxRune {
			deleted++
		} else {
			chars[i-deleted] = chars[i]
		}
	}
	chars = chars[:len(chars)-deleted]
	return string(chars)
}

func main() {
	// Site #1: write CMD
	l1 := NewRList()
	l1.InsertChar('C')
	l1.InsertChar('M')
	l1.InsertChar('D')
	// Create new sites
	l2 := l1.Fork()
	l3 := l2.Fork()
	// Site #1: CMD --> CTRL
	l1.DeleteChar()
	l1.DeleteChar()
	l1.InsertChar('T')
	l1.InsertChar('R')
	l1.InsertChar('L')
	// Site #2: CMD --> CMDALT
	l2.InsertChar('A')
	l2.InsertChar('L')
	l2.InsertChar('T')
	// Site #3: CMD --> CMDDEL
	l3.InsertChar('D')
	l3.InsertChar('E')
	l3.InsertChar('L')
	// Print lists
	fmt.Println(l1.AsString())
	fmt.Println(l2.AsString())
	fmt.Println(l3.AsString())
}
