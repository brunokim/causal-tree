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
