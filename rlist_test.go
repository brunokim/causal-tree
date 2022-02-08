package crdt

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

func toJSON(v interface{}) string {
	bs, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(bs)
}

func TestRList(t *testing.T) {
	defer func(oldUUIDv1 func() uuid.UUID) { uuidv1 = oldUUIDv1 }(uuidv1)
	var numUUID int
	uuidv1 = func() uuid.UUID {
		numUUID++
		// Random UUIDv1, except for incremental time bits.
		return uuid.MustParse(fmt.Sprintf("0000%04x-8891-11ec-a04c-67855c00505b", numUUID))
	}

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
	if s1 := l1.AsString(); s1 != "CTRL" {
		t.Errorf("1: l1 = %q, want %q", s1, "CTRL")
	}
	// Site #2: CMD --> CMDALT
	l2.InsertChar('A')
	l2.InsertChar('L')
	l2.InsertChar('T')
	if s2 := l2.AsString(); s2 != "CMDALT" {
		t.Errorf("2: l2 = %q, want %q", s2, "CMDALT")
	}
	// Site #3: CMD --> CMDDEL
	l3.InsertChar('D')
	l3.InsertChar('E')
	l3.InsertChar('L')
	if s3 := l3.AsString(); s3 != "CMDDEL" {
		t.Errorf("3: l3 = %q, want %q", s3, "CMDDEL")
	}
	// Merge site #2 into #1 --> CTRLALT
	l1.Merge(l2)
	if s1 := l1.AsString(); s1 != "CTRLALT" {
		t.Errorf("4: l1 = %q, want %q", s1, "CTRLALT")
	}
	// Merge site #3 into #1 --> CTRLALTDEL
	l1.Merge(l3)
	if s1 := l1.AsString(); s1 != "CTRLALTDEL" {
		t.Errorf("5: l1 = %q, want %q", s1, "CTRLALTDEL")
	}
	// Merge site #2 into #3 --> CMDALTDEL
	l3.Merge(l2)
	if s3 := l3.AsString(); s3 != "CMDALTDEL" {
		t.Errorf("3: l3 = %q, want %q", s3, "CMDALTDEL")
	}
	// Merge site #1 into #3 --> CTRLALTDEL
	l3.Merge(l1)
	if s3 := l3.AsString(); s3 != "CTRLALTDEL" {
		t.Errorf("3: l3 = %q, want %q", s3, "CTRLALTDEL")
	}
	fmt.Println(toJSON(l3))
}
