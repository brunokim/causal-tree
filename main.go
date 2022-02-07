package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"unicode"

	"github.com/google/uuid"
)

// Based on http://archagon.net/blog/2018/03/24/data-laced-with-history/

var (
	uuidv1 = randomUUIDv1 // For testing
)

// +-----------------------+
// | Basic data structures |
// +-----------------------+

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
	// Site is the index in the sitemap of the site that created an atom.
	Site uint16
	// Index is the order of creation of this atom in the given site.
	Index uint32
	// Timestamp is the Lamport timestamp of the site when the atom was created.
	Timestamp uint32
}

// AtomValue is a list operation.
type AtomValue interface {
	json.Marshaler
	// AtomPriority returns where this atom should be placed compared with its siblings.
	AtomPriority() int
}

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

// Returns the index where a site is (or should be) in the sitemap.
func siteIndex(sitemap []uuid.UUID, siteID uuid.UUID) int {
	return sort.Search(len(sitemap), func(i int) bool {
		return bytes.Compare(sitemap[i][:], siteID[:]) >= 0
	})
}

// Returns the index of an atom within the weave.
func (l *RList) atomIndex(atomID AtomID) int {
	for i, atom := range l.Weave {
		if atom.ID == atomID {
			return i
		}
	}
	return len(l.Weave)
}

// Inserts an atom in the given weave index.
func (l *RList) insertAtom(atom Atom, i int) {
	l.Weave = append(l.Weave, Atom{})
	copy(l.Weave[i+1:], l.Weave[i:])
	l.Weave[i] = atom
}

// +----------+
// | Ordering |
// +----------+

// Compare returns the relative order between atom IDs.
func (id AtomID) Compare(other AtomID) int {
	// Ascending according to timestamp (older first)
	if id.Timestamp < other.Timestamp {
		return -1
	}
	if id.Timestamp > other.Timestamp {
		return +1
	}
	// Descending according to site (younger first)
	if id.Site > other.Site {
		return -1
	}
	if id.Site < other.Site {
		return +1
	}
	return 0
}

// Compare returns the relative order between atoms.
func (a Atom) Compare(other Atom) int {
	// Ascending according to priority.
	if a.Value.AtomPriority() < other.Value.AtomPriority() {
		return -1
	}
	if a.Value.AtomPriority() > other.Value.AtomPriority() {
		return +1
	}
	return a.ID.Compare(other.ID)
}

// +------+
// | Fork |
// +------+

// Fork a replicated list into an independent object.
func (l *RList) Fork() *RList {
	if len(l.Sitemap)-1 >= math.MaxUint16 {
		panic("fork: reached limit of sites")
	}
	newSiteID := uuidv1()
	n := len(l.Sitemap)
	i := siteIndex(l.Sitemap, newSiteID)
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

// +-------+
// | Merge |
// +-------+

func mergeSitemaps(s1, s2 []uuid.UUID) []uuid.UUID {
	if len(s1) < len(s2) {
		s1, s2 = s2, s1
	}
	s := make([]uuid.UUID, len(s1), len(s1)+len(s2))
	copy(s, s1)
	for _, site := range s2 {
		i := sort.Search(len(s), func(i int) bool {
			return bytes.Compare(s[i][:], site[:]) >= 0
		})
		if i < len(s) && s[i] == site {
			continue
		}
		if i == len(s) {
			s = append(s, site)
		} else {
			s = append(s, uuid.Nil)
			copy(s[i+1:], s[i:])
			s[i] = site
		}
	}
	return s
}

func (a Atom) remapSite(m map[uint16]uint16) Atom {
	return Atom{
		ID:    a.ID.remapSite(m),
		Cause: a.Cause.remapSite(m),
		Value: a.Value,
	}
}

func (id AtomID) remapSite(m map[uint16]uint16) AtomID {
	newSite, ok := m[id.Site]
	if !ok {
		return id
	}
	return AtomID{
		Site:      newSite,
		Index:     id.Index,
		Timestamp: id.Timestamp,
	}
}

// Merge updates the current state with that of another remote list.
func (l *RList) Merge(remote *RList) {
	// 1. Merge sitemaps.
	sitemap := mergeSitemaps(l.Sitemap, remote.Sitemap)
	// 2. Compute site index remapping.
	localRemap := make(map[uint16]uint16)
	remoteRemap := make(map[uint16]uint16)
	for i, site := range l.Sitemap {
		j := siteIndex(sitemap, site)
		if i != j {
			localRemap[uint16(i)] = uint16(j)
		}
	}
	for i, site := range remote.Sitemap {
		j := siteIndex(sitemap, site)
		if i != j {
			remoteRemap[uint16(i)] = uint16(j)
		}
	}
	// 3. Remap atoms from local.
	yarns := make([][]Atom, len(sitemap))
	for i, yarn := range l.Yarns {
		ii, ok := localRemap[uint16(i)]
		if !ok {
			ii = uint16(i)
		}
		yarns[ii] = make([]Atom, len(yarn))
		for j, atom := range yarn {
			yarns[ii][j] = atom.remapSite(localRemap)
		}
	}
	for i, atom := range l.Weave {
		l.Weave[i] = atom.remapSite(localRemap)
	}
	// 4. Insert atoms from remote.
	for i, yarn := range remote.Yarns {
		ii, ok := remoteRemap[uint16(i)]
		if !ok {
			ii = uint16(i)
		}
		startIndex := len(yarns[ii])
		if len(yarn) > len(yarns[ii]) {
			// Grow yarn to accomodate remote atoms.
			yarns[ii] = append(yarns[ii], make([]Atom, len(yarn)-len(yarns[ii]))...)
		}
		for j := startIndex; j < len(yarn); j++ {
			atom := yarn[j].remapSite(remoteRemap)
			yarns[ii][j] = atom
			// Insert atom in local weave.
			// BUG: parent may not yet be inserted in weave, if it's from a remote yarn not yet integrated.
			parentIndex := l.atomIndex(atom.Cause)
			blockEnd, siblingIndices := l.causalBlock(parentIndex)
			insertionIndex := blockEnd
			// Insert atom in reversed order compared to siblings.
			// NOTE: not necessary to use binary search here, because iterating over siblings is strictly faster than it takes
			// to list them.
			for _, siblingIndex := range siblingIndices {
				sibling := l.Weave[siblingIndex]
				if atom.Compare(sibling) >= 0 {
					insertionIndex = siblingIndex
					break
				}
			}
			l.insertAtom(atom, insertionIndex)
			if parentIndex < int(l.Cursor) {
				l.Cursor++
			}
		}
	}
	l.Sitemap = sitemap
	l.Yarns = yarns
	if l.Timestamp < remote.Timestamp {
		l.Timestamp = remote.Timestamp
	}
	l.Timestamp++
}

// Returns the upper bound (exclusive) of head's causal block, plus its children's indices.
//
// The causal block is defined as the contiguous range containing all of head's descendants.
func (l *RList) causalBlock(headIndex int) (int, []int) {
	head := l.Weave[headIndex]
	var childIndices []int
	for i := headIndex + 1; i < len(l.Weave); i++ {
		atom := l.Weave[i]
		if atom.Cause == head.ID {
			childIndices = append(childIndices, i)
			continue
		}
		parentTimestamp := atom.Cause.Timestamp
		if parentTimestamp < head.ID.Timestamp {
			// First atom whose parent has a lower timestamp (older) than head is the end
			// of the causal block.
			return i, childIndices
		}
	}
	return len(l.Weave), childIndices
}

// +------------+
// | Operations |
// +------------+

func (l *RList) insertAtomAtCursor(atom Atom) {
	l.insertAtom(atom, int(l.Cursor))
}

func (l *RList) addAtom(value AtomValue) {
	l.Timestamp++
	if l.Timestamp == 0 {
		// Overflow
		panic("appending atom: reached limit of states")
	}
	i := siteIndex(l.Sitemap, l.SiteID)
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
	l.insertAtomAtCursor(atom)
	l.Yarns[i] = append(l.Yarns[i], atom)
}

// +--------------------------+
// | Operations - Insert char |
// +--------------------------+

// InsertChar represents insertion of a char to the right of another atom.
type InsertChar struct {
	// Char inserted in list.
	Char rune
}

func (v InsertChar) AtomPriority() int { return 0 }
func (v InsertChar) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"insert %c"`, v.Char)), nil
}

// InsertCharAfter inserts a char after the cursor position.
func (l *RList) InsertChar(ch rune) {
	l.addAtom(InsertChar{ch})
	l.Cursor++
}

// +---------------------+
// | Operations - Delete |
// +---------------------+

// Delete represents deleting an element from the list.
type Delete struct{}

func (v Delete) AtomPriority() int { return 100 }
func (v Delete) MarshalJSON() ([]byte, error) {
	return []byte(`"delete"`), nil
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

// +------------+
// | Conversion |
// +------------+

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

// +-----------+
// | Utilities |
// +-----------+

// Provides a random MAC address.
func randomMAC() []byte {
	mac := make([]byte, 6)
	if _, err := io.ReadFull(rand.Reader, mac); err != nil {
		panic(err.Error())
	}
	return mac
}

// Create UUIDv1, using local timestamp as lower bits and random MAC.
func randomUUIDv1() uuid.UUID {
	uuid.SetNodeID(randomMAC())
	id, err := uuid.NewUUID()
	if err != nil {
		panic(fmt.Sprintf("creating UUIDv1: %v", err))
	}
	return id
}

func toJSON(v interface{}) string {
	bs, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(bs)
}

// +------+
// | Test |
// +------+

func main() {
	//
	//  C - T - R - L
	//   `- M - D - A - L - T
	//      |   |`- D - E - L
	//      x   x
	//
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
	// Merge site #2 into #1 --> CTRLALT
	l1.Merge(l2)
	fmt.Println(l1.AsString())
	// Merge site #3 into #1 --> CTRLALTDEL
	l1.Merge(l3)
	fmt.Println(l1.AsString())
	// Merge site #2 into #3 --> CMDALTDEL
	l3.Merge(l2)
	fmt.Println(l3.AsString())
	// Merge site #1 into #3 --> CTRLALTDEL
	l3.Merge(l1)
	fmt.Println(l3.AsString())
	fmt.Println(toJSON(l3))
}
