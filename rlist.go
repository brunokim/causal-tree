package crdt

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
	// Or: the atom index on its site's yarn.
	Index uint32
	// Timestamp is the site's Lamport timestamp when the atom was created.
	Timestamp uint32
}

// AtomValue is a list operation.
type AtomValue interface {
	json.Marshaler
	// AtomPriority returns where this atom should be placed compared with its siblings.
	AtomPriority() int
}

// RList is a replicated list data structure.
//
// This data structure allows for 64K sites and 4G atoms in total.
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
		Timestamp: 1, // Timestamp 0 is considered invalid.
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

// +--------+
// | String |
// +--------+

func (id AtomID) String() string {
	return fmt.Sprintf("S%d@T%02d", id.Site, id.Timestamp)
}

func (a Atom) String() string {
	return fmt.Sprintf("Atom(%v,%v,%v)", a.ID, a.Cause, a.Value)
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
	i := siteIndex(l.Sitemap, newSiteID)
	// Insert empty yarn in local position.
	l.Yarns = append(l.Yarns, nil)
	copy(l.Yarns[i+1:], l.Yarns[i:])
	l.Yarns[i] = nil
	// Insert site ID into local sitemap.
	l.Sitemap = append(l.Sitemap, uuid.Nil)
	copy(l.Sitemap[i+1:], l.Sitemap[i:])
	l.Sitemap[i] = newSiteID
	// Copy data to remote list.
	n := len(l.Sitemap)
	l.Timestamp++
	remote := &RList{
		Weave:     make([]Atom, len(l.Weave)),
		Cursor:    l.Cursor,
		Yarns:     make([][]Atom, n),
		Sitemap:   make([]uuid.UUID, n),
		SiteID:    newSiteID,
		Timestamp: l.Timestamp,
	}
	copy(remote.Weave, l.Weave)
	for i, yarn := range l.Yarns {
		remote.Yarns[i] = make([]Atom, len(yarn))
		copy(remote.Yarns[i], yarn)
	}
	// BUG: need to remap atoms if new site index is != n
	copy(remote.Sitemap, l.Sitemap)
	return remote
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
		s = append(s, uuid.Nil)
		copy(s[i+1:], s[i:])
		s[i] = site
	}
	return s
}

// -----

// Map storing conversion between indices.
// Conversion from an index to itself are not stored.
// Queries for an index that was not inserted or stored return the index itself.
type indexMap map[int]int

func (m indexMap) set(i, j int) {
	if i != j {
		m[i] = j
	}
}

func (m indexMap) get(i int) int {
	j, ok := m[i]
	if !ok {
		return i
	}
	return j
}

// -----

func (a Atom) remapSite(m indexMap) Atom {
	return Atom{
		ID:    a.ID.remapSite(m),
		Cause: a.Cause.remapSite(m),
		Value: a.Value,
	}
}

func (id AtomID) remapSite(m indexMap) AtomID {
	return AtomID{
		Site:      uint16(m.get(int(id.Site))),
		Index:     id.Index,
		Timestamp: id.Timestamp,
	}
}

// -----

func mergeWeaves(w1, w2 []Atom) []Atom {
	var i, j int
	var weave []Atom
	for i < len(w1) && j < len(w2) {
		a1, a2 := w1[i], w2[j]
		if a1 == a2 {
			// Atoms are equal, append it to the weave.
			weave = append(weave, a1)
			i++
			j++
			continue
		}
		if a1.ID.Site == a2.ID.Site {
			// Atoms are from the same site and can be compared by timestamp.
			// Insert younger one (larger timestamp) in weave.
			if a1.ID.Timestamp < a2.ID.Timestamp {
				weave = append(weave, a2)
				j++
			} else {
				weave = append(weave, a1)
				i++
			}
		} else {
			// Atoms are concurrent; append first causal block, according to heads' order.
			if a1.Compare(a2) >= 0 {
				n1, _ := causalBlock(w1, i)
				weave = append(weave, w1[i:n1]...)
				i = n1
			} else {
				n2, _ := causalBlock(w2, j)
				weave = append(weave, w2[j:n2]...)
				j = n2
			}
		}
	}
	if i < len(w1) {
		weave = append(weave, w1[i:]...)
	}
	if j < len(w2) {
		weave = append(weave, w2[j:]...)
	}
	return weave
}

// Merge updates the current state with that of another remote list.
// Note that merge does not move the cursor.
func (l *RList) Merge(remote *RList) {
	// 1. Merge sitemaps.
	sitemap := mergeSitemaps(l.Sitemap, remote.Sitemap)
	// 2. Compute site index remapping.
	localRemap := make(indexMap)
	remoteRemap := make(indexMap)
	for i, site := range l.Sitemap {
		localRemap.set(i, siteIndex(sitemap, site))
	}
	for i, site := range remote.Sitemap {
		remoteRemap.set(i, siteIndex(sitemap, site))
	}
	// 3. Remap atoms from local.
	yarns := make([][]Atom, len(sitemap))
	for i, yarn := range l.Yarns {
		i := localRemap.get(i)
		yarns[i] = make([]Atom, len(yarn))
		for j, atom := range yarn {
			yarns[i][j] = atom.remapSite(localRemap)
		}
	}
	for i, atom := range l.Weave {
		l.Weave[i] = atom.remapSite(localRemap)
	}
	cursorID := l.Weave[l.Cursor-1].ID
	// 4. Merge yarns.
	for i, yarn := range remote.Yarns {
		i := remoteRemap.get(i)
		startIndex := len(yarns[i])
		if len(yarn) > len(yarns[i]) {
			// Grow yarn to accomodate remote atoms.
			yarns[i] = append(yarns[i], make([]Atom, len(yarn)-len(yarns[i]))...)
		}
		for j := startIndex; j < len(yarn); j++ {
			atom := yarn[j].remapSite(remoteRemap)
			yarns[i][j] = atom
		}
	}
	// 5. Merge weaves.
	remoteWeave := make([]Atom, len(remote.Weave))
	for i, atom := range remote.Weave {
		remoteWeave[i] = atom.remapSite(remoteRemap)
	}
	l.Weave = mergeWeaves(l.Weave, remoteWeave)
	//
	l.Yarns = yarns
	l.Sitemap = sitemap
	if l.Timestamp < remote.Timestamp {
		l.Timestamp = remote.Timestamp
	}
	l.Cursor = uint32(l.atomIndex(cursorID) + 1)
	l.Timestamp++
}

// Returns the upper bound (exclusive) of head's causal block, plus its children's indices.
//
// The causal block is defined as the contiguous range containing all of head's descendants.
func (l *RList) causalBlock(headIndex int) (int, []int) {
	return causalBlock(l.Weave, headIndex)
}

func causalBlock(weave []Atom, headIndex int) (int, []int) {
	head := weave[headIndex]
	var childIndices []int
	for i := headIndex + 1; i < len(weave); i++ {
		atom := weave[i]
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
	return len(weave), childIndices
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
func (v InsertChar) String() string { return string([]rune{v.Char}) }

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
func (v Delete) String() string { return "⌫ " }

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
