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

// Clone copies all information of a list without creating a new site.
func (l *RList) Clone() *RList {
	n := len(l.Sitemap)
	remote := &RList{
		Weave:     make([]Atom, len(l.Weave)),
		Cursor:    l.Cursor,
		Yarns:     make([][]Atom, n),
		Sitemap:   make([]uuid.UUID, n),
		SiteID:    l.SiteID,
		Timestamp: l.Timestamp,
	}
	copy(remote.Weave, l.Weave)
	for i, yarn := range l.Yarns {
		remote.Yarns[i] = make([]Atom, len(yarn))
		copy(remote.Yarns[i], yarn)
	}
	copy(remote.Sitemap, l.Sitemap)
	return remote
}
