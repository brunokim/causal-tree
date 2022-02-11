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

func setupUUIDs(uuids []uuid.UUID) func() {
	var i int
	oldUUIDv1 := uuidv1
	teardown := func() { uuidv1 = oldUUIDv1 }
	uuidv1 = func() uuid.UUID {
		uuid := uuids[i]
		i++
		return uuid
	}
	return teardown
}

func TestRList(t *testing.T) {
	teardown := setupUUIDs([]uuid.UUID{
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000002-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000003-8891-11ec-a04c-67855c00505b"),
	})
	defer teardown()

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
		t.Errorf("6: l3 = %q, want %q", s3, "CMDALTDEL")
	}
	// Merge site #1 into #3 --> CTRLALTDEL
	l3.Merge(l1)
	if s3 := l3.AsString(); s3 != "CTRLALTDEL" {
		t.Errorf("7: l3 = %q, want %q", s3, "CTRLALTDEL")
	}
	fmt.Println(toJSON(l3))
}

func TestBackwardsClock(t *testing.T) {
	teardown := setupUUIDs([]uuid.UUID{
		// UUIDs don't progress with increasing timestamp: 1,5,2,4,3
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000005-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000002-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000004-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000003-8891-11ec-a04c-67855c00505b"),
	})
	defer teardown()

	// C - O - O - P
	//      `- D - E - . - I - O
	//         |   |
	//         x   x
	// Create sites #1, #2, #3: C, CODE
	l1 := NewRList()
	l1.InsertChar('C')
	l2 := l1.Fork()
	l2.InsertChar('O')
	l2.InsertChar('D')
	l2.InsertChar('E')
	l3 := l2.Fork()
	// Create site #4 from #3: CODE --> CODE.IO
	l4 := l3.Fork()
	l4.InsertChar('.')
	l4.InsertChar('I')
	l4.InsertChar('O')
	if s4 := l4.AsString(); s4 != "CODE.IO" {
		t.Errorf("2: l4 = %q, want %q", s4, "CODE.IO")
	}
	// Site #3: CODE --> COOP
	l3.DeleteChar()
	l3.DeleteChar()
	l3.InsertChar('O')
	l3.InsertChar('P')
	if s3 := l3.AsString(); s3 != "COOP" {
		t.Errorf("3: l3 = %q, want %q", s3, "COOP")
	}
	// Copy l3 into l5
	l5 := l3.Fork()
	// Merge site #4 into #3
	l3.Merge(l4)
	if s3 := l3.AsString(); s3 != "COOP.IO" {
		t.Errorf("4: l3 = %q, want %q", s3, "COOP.IO")
	}
	// Merge site #5 (copy of #1 before merge) into #4
	l4.Merge(l5)
	if s4 := l4.AsString(); s4 != "COOP.IO" {
		t.Errorf("5: l4 = %q, want %q", s4, "COOP.IO")
	}
	// Ensure other streams are not changed.
	if s1 := l1.AsString(); s1 != "C" {
		t.Errorf("6: l1 = %q, want %q", s1, "C")
	}
	if s2 := l2.AsString(); s2 != "CODE" {
		t.Errorf("7: l2 = %q, want %q", s2, "CODE")
	}
}

func TestUnknownRemoteYarn(t *testing.T) {
	teardown := setupUUIDs([]uuid.UUID{
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000002-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000003-8891-11ec-a04c-67855c00505b"),
	})
	defer teardown()

	// Site #1: A - B -----------------------.- *
	// Site #2:      `- C - D -------.- G - H
	// Site #3:              `- E - F
	// Create site #1
	l1 := NewRList()
	l1.InsertChar('A')
	l1.InsertChar('B')
	// Create site #2 from #1: AB --> ABCD
	l2 := l1.Fork()
	l2.InsertChar('C')
	l2.InsertChar('D')
	if s2 := l2.AsString(); s2 != "ABCD" {
		t.Errorf("2: l2 = %q, want %q", s2, "ABCD")
	}
	// Site #3: ABCD --> ABCDEF
	l3 := l2.Fork()
	l3.InsertChar('E')
	l3.InsertChar('F')
	if s3 := l3.AsString(); s3 != "ABCDEF" {
		t.Errorf("3: l3 = %q, want %q", s3, "ABCDEF")
	}
	// Merge site #3 into #2: ABCDEF --> ABCDEFGH
	l2.Merge(l3)
	l2.InsertChar('G')
	l2.InsertChar('H')
	if s2 := l2.AsString(); s2 != "ABCDEFGH" {
		t.Errorf("4: l2 = %q, want %q", s2, "ABCDEFGH")
	}
	// Merge site #2 into #1
	l1.Merge(l2)
	if s1 := l1.AsString(); s1 != "ABCDEFGH" {
		t.Errorf("5: l1 = %q, want %q", s1, "ABCDEFGH")
	}
}
