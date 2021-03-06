/*
Package crdt provides primitives to operate on replicated data types.

Replicated data types are structured such that they can be copied across multiple sites
in a distributed environment, mutated independently at each site, and they still may be
merged back without conflicts.

This implementation is based on the Causal Tree structure proposed by Victor Grishchenko [1],
following the excellent explanation by Archagon [2].

[1]: GRISCHENKO, VICTOR. Causal trees: towards real-time read-write hypertext.
[2]: http://archagon.net/blog/2018/03/24/data-laced-with-history/
*/
package crdt

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"

	"github.com/google/uuid"
)

/*
In a causal tree, each operation on the data structure is represented by an atom, which
has a single other atom as its cause, thus creating a tree shape.
When inserting a char, for example, its cause is the char to its left.

  # BEGIN ASCII ART

  T <- H <- I <- S <- _ <- I <- S <- _ <- N <- I <- C <- E
                                     ^
                                     '-- V <- E <- R <- Y <- _

  # END ASCII ART
  # ALT TEXT: Sequence of letters with arrows between them, representing atoms and their causes.
              In the first line it reads "THIS_IS_NICE", and in the second the string "VERY_" points
              to the space after "IS". This represents an insertion, thus the whole tree should be
              read as "THIS_IS_VERY_NICE".

Instead of using a pointer to reference the causing operation, references simply hold an atom
ID containing the origin site and the (local) timestamp of creation.
Atoms are then organized in an array to improve memory locality, at the expanse of having to
search linearly for a given atom ID.

By sorting the array such that atoms from the same site and time are mostly contiguous, this search
operation is not terribly costly, and the array reads almost like the structure being represented.

  # BEGIN ASCII ART

   id cause                                  .--------------------------------------.
   |  |                                      .--------.                             |
   v  v                                      v        |                             |
  .-----.-----.-----.-----.-----.-----.-----.-----.-----.-----.-----.-----.-----.-----.-----.-----.-----.
  |01|__|02|01|03|02|04|03|05|04|06|05|07|06|08|07|13|08|14|13|15|14|16|15|17|16|09|08|10|09|11|10|12|11|
  +-----+-----+-----+-----+-----+-----+-----+-----+-----+-----+-----+-----+-----+-----+-----+-----+-----+
  |  T  |  H  |  I  |  S  |  _  |  I  |  S  |  _  |  V  |  E  |  R  |  Y  |  _  |  N  |  I  |  C  |  E  |
  '-----'-----'-----'-----'-----'-----'-----'-----'-----'-----'-----'-----'-----'-----'-----'-----'-----'

  # END ASCII ART
  # ALT TEXT: Sequence of atoms in an array, representing the same string as before. Each atom is composed
              of three elements: ID, cause, and content. IDs are given sequentially, from 01 to 17.
              From 01 to 08, all atoms are sorted by ID, with contents that spell "THIS_IS_".
              To the right of 08, we have the sequence of IDs from 13 to 17, with contents that spell
              "VERY_". The cause of ID 13 is the atom 08.
              Finally, to the right of ID 17 is the sequence of atoms from 09 to 12, with contents that
              spell "NICE". The cause of ID 09 is also the atom 08.
              The first atom has no cause, and all others have as cause the atom to its left.
*/

var (
	uuidv1 = randomUUIDv1 // Stubbed for mocking in mocks_test.go
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
	// ValidateChild checks whether the given value can be appended as a child.
	ValidateChild(child AtomValue) error
}

// RList is a replicated list data structure.
//
// This data structure allows for 64K sites and 4G atoms in total.
type RList struct {
	// Weave is the flat representation of a causal tree.
	Weave []Atom
	// Cursor is the ID of the causing atom for the next operation.
	Cursor AtomID
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
		Cursor:    AtomID{},
		Yarns:     [][]Atom{nil},
		Sitemap:   []uuid.UUID{siteID},
		SiteID:    siteID,
		Timestamp: 1, // Timestamp 0 is considered invalid.
	}
}

// Returns the index where a site is (or should be) in the sitemap.
//
// Time complexity: O(log(sites))
func siteIndex(sitemap []uuid.UUID, siteID uuid.UUID) int {
	return sort.Search(len(sitemap), func(i int) bool {
		return bytes.Compare(sitemap[i][:], siteID[:]) >= 0
	})
}

// Returns the index of an atom within the weave.
//
// Time complexity: O(atoms)
func (l *RList) atomIndex(atomID AtomID) int {
	if atomID.Timestamp == 0 {
		return -1
	}
	for i, atom := range l.Weave {
		if atom.ID == atomID {
			return i
		}
	}
	return len(l.Weave)
}

// Gets an atom from yarns.
//
// Time complexity: O(1)
func (l *RList) getAtom(atomID AtomID) Atom {
	return l.Yarns[atomID.Site][atomID.Index]
}

// Inserts an atom in the given weave index.
//
// Time complexity: O(atoms)
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

// +---------------+
// | Remap indices |
// +---------------+

// Map storing conversion between indices.
// Conversion from an index to itself are not stored.
// An empty map represents an identity mapping, where every index maps to itself.
// Queries for an index that was not inserted or stored return the same index.
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

// +------+
// | Fork |
// +------+

// Fork a replicated list into an independent object.
//
// Time complexity: O(atoms)
func (l *RList) Fork() (*RList, error) {
	if len(l.Sitemap)-1 >= math.MaxUint16 {
		return nil, ErrSiteLimitExceeded
	}
	newSiteID := uuidv1()
	i := siteIndex(l.Sitemap, newSiteID)
	if i == len(l.Sitemap) {
		l.Yarns = append(l.Yarns, nil)
		l.Sitemap = append(l.Sitemap, newSiteID)
	} else {
		// Remap atoms in yarns and weave.
		localRemap := make(indexMap)
		for j := i; j < len(l.Sitemap); j++ {
			localRemap.set(j, j+1)
		}
		for i, yarn := range l.Yarns {
			for j, atom := range yarn {
				l.Yarns[i][j] = atom.remapSite(localRemap)
			}
		}
		for i, atom := range l.Weave {
			l.Weave[i] = atom.remapSite(localRemap)
		}
		l.Cursor = l.Cursor.remapSite(localRemap)
		// Insert empty yarn in local position.
		l.Yarns = append(l.Yarns, nil)
		copy(l.Yarns[i+1:], l.Yarns[i:])
		l.Yarns[i] = nil
		// Insert site ID into local sitemap.
		l.Sitemap = append(l.Sitemap, uuid.Nil)
		copy(l.Sitemap[i+1:], l.Sitemap[i:])
		l.Sitemap[i] = newSiteID
	}
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
	copy(remote.Sitemap, l.Sitemap)
	return remote, nil
}

// +-------+
// | Merge |
// +-------+

// Time complexity: O(sites)
func mergeSitemaps(s1, s2 []uuid.UUID) []uuid.UUID {
	var i, j int
	s := make([]uuid.UUID, 0, len(s1)+len(s2))
	for i < len(s1) && j < len(s2) {
		id1, id2 := s1[i], s2[j]
		order := bytes.Compare(id1[:], id2[:])
		if order < 0 {
			s = append(s, id1)
			i++
		} else if order > 0 {
			s = append(s, id2)
			j++
		} else {
			s = append(s, id1)
			i++
			j++
		}
	}
	if i < len(s1) {
		s = append(s, s1[i:]...)
	}
	if j < len(s2) {
		s = append(s, s2[j:]...)
	}
	return s
}

// Time complexity: O(atoms)
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
				n1 := i + causalBlockSize(w1[i:])
				weave = append(weave, w1[i:n1]...)
				i = n1
			} else {
				n2 := j + causalBlockSize(w2[j:])
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
//
// Time complexity: O(atoms^2 + sites*log(sites))
func (l *RList) Merge(remote *RList) {
	// 1. Merge sitemaps.
	// Time complexity: O(sites)
	sitemap := mergeSitemaps(l.Sitemap, remote.Sitemap)

	// 2. Compute site index remapping.
	// Time complexity: O(sites*log(sites))
	localRemap := make(indexMap)
	remoteRemap := make(indexMap)
	for i, site := range l.Sitemap {
		localRemap.set(i, siteIndex(sitemap, site))
	}
	for i, site := range remote.Sitemap {
		remoteRemap.set(i, siteIndex(sitemap, site))
	}

	// 3. Remap atoms from local.
	// Time complexity: O(atoms)
	yarns := make([][]Atom, len(sitemap))
	if len(localRemap) > 0 {
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
	} else {
		for i, yarn := range l.Yarns {
			yarns[i] = make([]Atom, len(yarn))
			copy(yarns[i], yarn)
		}
	}

	// 4. Merge yarns.
	// Time complexity: O(atoms)
	for i, yarn := range remote.Yarns {
		i := remoteRemap.get(i)
		start := len(yarns[i])
		end := len(yarn)
		if end > start {
			// Grow yarn to accomodate remote atoms.
			yarns[i] = append(yarns[i], make([]Atom, end-start)...)
		}
		for j := start; j < end; j++ {
			atom := yarn[j].remapSite(remoteRemap)
			yarns[i][j] = atom
		}
	}

	// 5. Merge weaves.
	// Time complexity: O(atoms)
	remoteWeave := make([]Atom, len(remote.Weave))
	for i, atom := range remote.Weave {
		remoteWeave[i] = atom.remapSite(remoteRemap)
	}
	l.Weave = mergeWeaves(l.Weave, remoteWeave)

	// Move created stuff to this list.
	l.Yarns = yarns
	l.Sitemap = sitemap
	if l.Timestamp < remote.Timestamp {
		l.Timestamp = remote.Timestamp
	}
	l.Timestamp++

	// 6. Fix cursor if necessary.
	// Time complexity: O(atoms^2)
	l.Cursor = l.Cursor.remapSite(localRemap)
	l.fixDeletedCursor()
}

// -----

// Invokes the closure f with each atom of the causal block. Returns the number of atoms visited.
//
// The closure should return 'false' to cut the traversal short, as in a 'break' statement. Otherwise, return true.
//
// The causal block is defined as the contiguous range containing the head and all of its descendents.
//
// Time complexity: O(atoms), or, O(avg. block size)
func walkCausalBlock(block []Atom, f func(Atom) bool) int {
	if len(block) == 0 {
		return 0
	}
	head := block[0]
	for i, atom := range block[1:] {
		if atom.Cause.Timestamp < head.ID.Timestamp {
			// First atom whose parent has a lower timestamp (older) than head is the
			// end of the causal block.
			return i + 1
		}
		if !f(atom) {
			break
		}
	}
	return len(block)
}

// Invokes the closure f with each direct children of the block's head.
//
// The index i corresponds to the index on the causal block, not on the child's order.
func walkChildren(block []Atom, f func(Atom) bool) {
	walkCausalBlock(block, func(atom Atom) bool {
		if atom.Cause == block[0].ID {
			return f(atom)
		}
		return true
	})
}

// Returns the size of the causal block, including its head.
func causalBlockSize(block []Atom) int {
	return walkCausalBlock(block, func(atom Atom) bool { return true })
}

// Returns whether the atom is deleted.
//
// Time complexity: O(atoms), or, O(avg. block size)
func (l *RList) isDeleted(atomID AtomID) bool {
	i := l.atomIndex(atomID)
	if i < 0 {
		return false
	}
	var isDeleted bool
	walkChildren(l.Weave[i:], func(child Atom) bool {
		if _, ok := child.Value.(Delete); ok {
			isDeleted = true
			return false
		}
		// There's a child with lower priority than delete, so there can't be
		// any more delete atom ahead.
		if child.Value.AtomPriority() < (Delete{}).AtomPriority() {
			isDeleted = false
			return false
		}
		return true
	})
	return isDeleted
}

// Ensure list's cursor isn't deleted, finding the first non-deleted ancestor.
//
// Time complexity: O(atoms^2), or, O((avg. tree height) * (avg. block size))
func (l *RList) fixDeletedCursor() {
	for {
		if !l.isDeleted(l.Cursor) {
			break
		}
		l.Cursor = l.getAtom(l.Cursor).Cause
	}
}

// +-------------+
// + Time travel |
// +-------------+

// Weft is a clock that stores the timestamp of each site of a RList.
//
// In a distributed system it's not possible to observe the whole state at an absolute time,
// but we can view the site's state at each site time.
type Weft []uint32

// Compare returns -1, +1 and 0 if this is weft is less than, greater than, or concurrent
// to the other, respectively.
//
// It panics if wefts have different sizes.
func (w Weft) Compare(other Weft) int {
	if len(w) != len(other) {
		panic(fmt.Sprintf("wefts have different sizes: %d (%v) != %d (%v)", len(w), w, len(other), other))
	}
	var hasLess, hasGreater bool
	for i, t1 := range w {
		t2 := other[i]
		if t1 < t2 {
			hasLess = true
		} else if t1 > t2 {
			hasGreater = true
		}
	}
	if hasLess && hasGreater {
		return 0
	}
	if hasLess {
		return -1
	}
	if hasGreater {
		return +1
	}
	return 0
}

// The same as weft, but using yarn's indices instead of timestamps.
type indexWeft []int

// Returns whether the provided atom is present in the yarn's view.
// The nil atom is always in view.
func (ixs indexWeft) isInView(id AtomID) bool {
	return int(id.Index) < ixs[id.Site] || id.Timestamp == 0
}

// Checks that the weft is well-formed, not disconnecting atoms from their causes
// in other sites.
//
// Time complexity: O(atoms)
func (l *RList) checkWeft(weft Weft) (indexWeft, error) {
	if len(l.Yarns) != len(weft) {
		return nil, ErrWeftInvalidLength
	}
	// Initialize limits at each yarn.
	limits := make(indexWeft, len(weft))
	for i, yarn := range l.Yarns {
		limits[i] = len(yarn)
	}
	// Look for max timestamp at each yarn.
	for i, yarn := range l.Yarns {
		tmax := weft[i]
		for j, atom := range yarn {
			if atom.ID.Timestamp > tmax {
				limits[i] = j
				break
			}
		}
	}
	// Verify that all causes are present at the weft cut.
	for i, yarn := range l.Yarns {
		limit := limits[i]
		for _, atom := range yarn[:limit] {
			if !limits.isInView(atom.Cause) {
				return nil, ErrWeftDisconnected
			}
		}
	}
	return limits, nil
}

// Now returns the last known time at every site as a weft.
func (l *RList) Now() Weft {
	weft := make(Weft, len(l.Yarns))
	for i, yarn := range l.Yarns {
		n := len(yarn)
		if n == 0 {
			continue
		}
		weft[i] = yarn[n-1].ID.Timestamp
	}
	return weft
}

// ViewAt returns a view of the list in the provided time in the past, represented with a weft.
//
// Time complexity: O(atoms+sites)
func (l *RList) ViewAt(weft Weft) (*RList, error) {
	limits, err := l.checkWeft(weft)
	if err != nil {
		return nil, err
	}
	n := len(limits)
	yarns := make([][]Atom, n)
	for i, yarn := range l.Yarns {
		yarns[i] = make([]Atom, limits[i])
		copy(yarns[i], yarn)
	}
	weave := make([]Atom, 0, len(l.Weave))
	for _, atom := range l.Weave {
		if limits.isInView(atom.ID) {
			weave = append(weave, atom)
		}
	}
	sitemap := make([]uuid.UUID, n)
	copy(sitemap, l.Sitemap)
	// Set cursor, if it still exists in this view.
	cursor := l.Cursor
	if !limits.isInView(cursor) {
		cursor = AtomID{}
	}
	//
	i := siteIndex(l.Sitemap, l.SiteID)
	tmax := weft[i]
	view := &RList{
		Weave:     weave,
		Cursor:    cursor,
		Yarns:     yarns,
		Sitemap:   sitemap,
		SiteID:    l.SiteID,
		Timestamp: tmax,
	}
	return view, nil
}

// +---------------------+
// | Operations - Errors |
// +---------------------+

// Errors returned by RList operations
var (
	ErrSiteLimitExceeded  = errors.New("reached limit of sites: 2????? (65.536)")
	ErrStateLimitExceeded = errors.New("reached limit of states: 2???? (4.294.967.296)")
	ErrNoAtomToDelete     = errors.New("can't delete empty atom")
	ErrCursorOutOfRange   = errors.New("cursor index out of range")
	ErrWeftInvalidLength  = errors.New("weft length doesn't match with number of sites")
	ErrWeftDisconnected   = errors.New("weft disconnects some atom from its cause")
)

// +------------+
// | Operations |
// +------------+

// Time complexity: O(atoms), or, O(atoms + (avg. block size))
func (l *RList) insertAtomAtCursor(atom Atom) {
	if l.Cursor.Timestamp == 0 {
		// Cursor is at initial position.
		l.insertAtom(atom, 0)
		return
	}
	// Search for position in weave that atom should be inserted, in a way that it's sorted relative to
	// other children in descending order.
	//
	//                                  causal block of cursor
	//                      ------------------------------------------------
	// Weave:           ... [cursor] [child1] ... [child2] ... [child3] ... [not child]
	// Block indices:           0         1          c2'          c3'           end'
	// Weave indices:          c0        c1          c2           c3            end
	c0 := l.atomIndex(l.Cursor)
	var pos, i int
	walkCausalBlock(l.Weave[c0:], func(a Atom) bool {
		i++
		if a.Cause == l.Cursor && a.Compare(atom) < 0 && pos == 0 {
			// a is the first child smaller than atom.
			pos = i
		}
		return true
	})
	index := c0 + i + 1
	if pos > 0 {
		index = c0 + pos
	}
	l.insertAtom(atom, index)
}

// Inserts the atom as a child of the cursor, and returns its ID.
//
// Time complexity: O(atoms + log(sites))
func (l *RList) addAtom(value AtomValue) (AtomID, error) {
	l.Timestamp++
	if l.Timestamp == 0 {
		// Overflow
		return AtomID{}, ErrStateLimitExceeded
	}
	if l.Cursor.Timestamp > 0 {
		cursorAtom := l.getAtom(l.Cursor)
		if err := cursorAtom.Value.ValidateChild(value); err != nil {
			return AtomID{}, err
		}
	}
	i := siteIndex(l.Sitemap, l.SiteID)
	atomID := AtomID{
		Site:      uint16(i),
		Index:     uint32(len(l.Yarns[i])),
		Timestamp: l.Timestamp,
	}
	atom := Atom{
		ID:    atomID,
		Cause: l.Cursor,
		Value: value,
	}
	l.insertAtomAtCursor(atom)
	l.Yarns[i] = append(l.Yarns[i], atom)
	return atomID, nil
}

// +-------------------------+
// | Operations - Set cursor |
// +-------------------------+

// Time complexity: O(atoms)
func (l *RList) filterDeleted() []Atom {
	atoms := make([]Atom, len(l.Weave))
	copy(atoms, l.Weave)
	indices := make(map[AtomID]int)
	var hasDelete bool
	for i, atom := range l.Weave {
		indices[atom.ID] = i
		if _, ok := atom.Value.(Delete); ok {
			hasDelete = true
			// Deletion must always come after deleted atom, so
			// indices map must have the cause location.
			deletedAtomIdx := indices[atom.Cause]
			atoms[i] = Atom{}
			atoms[deletedAtomIdx] = Atom{}
		}
	}
	if !hasDelete {
		// Cheap optimization for case where there are no deletions.
		return atoms
	}
	// Move chars to fill in holes of empty atoms.
	deleted := 0
	for i, atom := range atoms {
		if atom.ID.Timestamp == 0 {
			deleted++
		} else {
			atoms[i-deleted] = atoms[i]
		}
	}
	atoms = atoms[:len(atoms)-deleted]
	return atoms
}

// Sets cursor to the given (list) position.
//
// To insert an atom at the beginning, use i = -1.
func (l *RList) SetCursor(i int) error {
	if i < 0 {
		if i == -1 {
			l.Cursor = AtomID{}
			return nil
		}
		return ErrCursorOutOfRange
	}
	atoms := l.filterDeleted()
	if i >= len(atoms) {
		return ErrCursorOutOfRange
	}
	l.Cursor = atoms[i].ID
	return nil
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
	return json.Marshal(fmt.Sprintf("insert %c", v.Char))
}
func (v InsertChar) String() string { return string([]rune{v.Char}) }

func (v InsertChar) ValidateChild(child AtomValue) error {
	switch child.(type) {
	case InsertChar:
		return nil
	case Delete:
		return nil
	default:
		return fmt.Errorf("invalid atom value after InsertChar: %T (%v)", child, child)
	}
}

// InsertChar inserts a char after the cursor position and advances the cursor.
func (l *RList) InsertChar(ch rune) error {
	atomID, err := l.addAtom(InsertChar{ch})
	if err != nil {
		return err
	}
	l.Cursor = atomID
	return nil
}

// InsertCharAt inserts a char after the given (list) position.
func (l *RList) InsertCharAt(ch rune, i int) error {
	if err := l.SetCursor(i); err != nil {
		return err
	}
	return l.InsertChar(ch)
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
func (v Delete) String() string { return "??? " }

func (v Delete) ValidateChild(child AtomValue) error {
	return fmt.Errorf("invalid atom value after Delete: %T (%v)", child, child)
}

// DeleteChar deletes the char at the cursor position, and relocates the cursor to its cause.
func (l *RList) DeleteChar() error {
	if l.Cursor.Timestamp == 0 {
		return ErrNoAtomToDelete
	}
	if _, err := l.addAtom(Delete{}); err != nil {
		return err
	}
	l.fixDeletedCursor()
	return nil
}

// DeleteCharAt deletes the char at the given (list) position.
func (l *RList) DeleteCharAt(i int) error {
	if err := l.SetCursor(i); err != nil {
		return err
	}
	return l.DeleteChar()
}

// +------------+
// | Conversion |
// +------------+

// AsString interprets list as a sequence of chars.
func (l *RList) AsString() string {
	atoms := l.filterDeleted()
	chars := make([]rune, len(atoms))
	for i, atom := range atoms {
		chars[i] = atom.Value.(InsertChar).Char
	}
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
