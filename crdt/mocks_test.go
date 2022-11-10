package crdt

import (
	"github.com/google/uuid"
)

// Mock UUID generation for testing. Returns a function to undo the mocking.
func MockUUIDs(uuids ...uuid.UUID) func() {
	var i int
	oldUUIDv1 := uuidv1
	undo := func() { uuidv1 = oldUUIDv1 }
	uuidv1 = func() uuid.UUID {
		uuid := uuids[i]
		i++
		return uuid
	}
	return undo
}

// Clone copies all information of a tree without creating a new site.
func (t *CausalTree) Clone() *CausalTree {
	n := len(t.Sitemap)
	remote := &CausalTree{
		Weave:     make([]Atom, len(t.Weave)),
		Cursor:    t.Cursor,
		Yarns:     make([][]Atom, n),
		Sitemap:   make([]uuid.UUID, n),
		SiteID:    t.SiteID,
		Timestamp: t.Timestamp,
	}
	copy(remote.Weave, t.Weave)
	for i, yarn := range t.Yarns {
		remote.Yarns[i] = make([]Atom, len(yarn))
		copy(remote.Yarns[i], yarn)
	}
	copy(remote.Sitemap, t.Sitemap)
	return remote
}
