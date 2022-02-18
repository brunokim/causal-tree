package crdt

import (
	"encoding/json"
	"fmt"
	"os"
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

func setupTestFile(name string) (*os.File, error) {
	os.MkdirAll("testdata", 0777)
	return os.Create(fmt.Sprintf("testdata/%s.jsonl", name))
}

// -----

// Tests are structured as a sequence of operations on a list of lists.
//
// This indirection allows us to perform some actions for every mutation, like
// dumping their internals to a file. Hopefully, it should also allow us to
// somehow fuzz list manipulation.
//
// Operations are:
//
// insertChar <local> <char> -- insert a char at cursor on list 'local'.
// deleteChar <local>        -- delete the char at cursor on list 'local'.
// fork <local> <remote>     -- fork list 'local' into list 'remote'.
// merge <local> <remote>    -- merge list 'remote' into list 'local'.
// check <local> <str>       -- check that the contents of 'local' spell 'str'.
//
// Lists are referred by their order of creation, NOT by their sitemap index.
// The fork operation requires specifying the correct index, even if it could be
// inferred from the number of already created lists, just to improve readability.

type operationType int

const (
	insertChar operationType = iota
	deleteChar
	fork
	merge
	check
)

type operation struct {
	op            operationType
	local, remote int
	char          rune
	str           string
}

func (op operation) String() string {
	switch op.op {
	case insertChar:
		return fmt.Sprintf("insert %c at list #%d", op.char, op.local)
	case deleteChar:
		return fmt.Sprintf("delete char from list #%d", op.local)
	case fork:
		return fmt.Sprintf("fork list #%d into list #%d", op.local, op.remote)
	case merge:
		return fmt.Sprintf("merge list #%d into list #%d", op.remote, op.local)
	}
	return ""
}

func runOperations(t *testing.T, ops []operation) []*RList {
	lists := []*RList{NewRList()}
	f, err := setupTestFile(t.Name())
	if err != nil {
		t.Log(err)
	}
	for i, op := range ops {
		list := lists[op.local]
		switch op.op {
		case insertChar:
			list.InsertChar(op.char)
		case deleteChar:
			list.DeleteChar()
		case fork:
			if op.remote != len(lists) {
				t.Fatalf("fork: expecting remote index %d, got %d", op.remote, len(lists))
			}
			lists = append(lists, list.Fork())
		case merge:
			list.Merge(lists[op.remote])
		case check:
			if s := list.AsString(); s != op.str {
				t.Errorf("%d: got list[%d] = %q, want %q", i, op.local, s, op.str)
			}
		}
		// Dump lists into testfile.
		if f != nil && op.op != check {
			bs, err := json.Marshal(map[string]interface{}{
				"Action": op.String(),
				"Sites":  lists,
			})
			if err != nil {
				t.Log(err)
				f.Close()
				f = nil
			} else {
				f.Write(bs)
				f.WriteString("\n")
			}
		}
	}
	if f != nil {
		f.Close()
	}
	return lists
}

// -----

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
	runOperations(t, []operation{
		// Site #0: write CMD
		{op: insertChar, local: 0, char: 'C'},
		{op: insertChar, local: 0, char: 'M'},
		{op: insertChar, local: 0, char: 'D'},
		// Create new sites
		{op: fork, local: 0, remote: 1},
		{op: fork, local: 1, remote: 2},
		// Site #0: CMD --> CTRL
		{op: deleteChar, local: 0},
		{op: deleteChar, local: 0},
		{op: insertChar, local: 0, char: 'T'},
		{op: insertChar, local: 0, char: 'R'},
		{op: insertChar, local: 0, char: 'L'},
		{op: check, local: 0, str: "CTRL"},
		// Site #1: CMD --> CMDALT
		{op: insertChar, local: 1, char: 'A'},
		{op: insertChar, local: 1, char: 'L'},
		{op: insertChar, local: 1, char: 'T'},
		{op: check, local: 1, str: "CMDALT"},
		// Site #2: CMD --> CMDDEL
		{op: insertChar, local: 2, char: 'D'},
		{op: insertChar, local: 2, char: 'E'},
		{op: insertChar, local: 2, char: 'L'},
		{op: check, local: 2, str: "CMDDEL"},
		// Merge site #1 into #0 --> CTRLALT
		{op: merge, local: 0, remote: 1},
		{op: check, local: 0, str: "CTRLALT"},
		// Merge site #2 into #0 --> CTRLALTDEL
		{op: merge, local: 0, remote: 2},
		{op: check, local: 0, str: "CTRLALTDEL"},
		// Merge site #1 into #2 --> CMDALTDEL
		{op: merge, local: 2, remote: 1},
		{op: check, local: 2, str: "CMDALTDEL"},
		// Merge site #0 into #2 --> CTRLALTDEL
		{op: merge, local: 2, remote: 0},
		{op: check, local: 2, str: "CTRLALTDEL"},
	})
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
	runOperations(t, []operation{
		// Create sites #0, #1, #2: C, CODE
		{op: insertChar, local: 0, char: 'C'},
		{op: fork, local: 0, remote: 1},
		{op: insertChar, local: 1, char: 'O'},
		{op: insertChar, local: 1, char: 'D'},
		{op: insertChar, local: 1, char: 'E'},
		{op: fork, local: 1, remote: 2},
		// Create site #3 from #2: CODE --> CODE.IO
		{op: fork, local: 2, remote: 3},
		{op: insertChar, local: 3, char: '.'},
		{op: insertChar, local: 3, char: 'I'},
		{op: insertChar, local: 3, char: 'O'},
		{op: check, local: 3, str: "CODE.IO"},
		// Site #2: CODE --> COOP
		{op: deleteChar, local: 2},
		{op: deleteChar, local: 2},
		{op: insertChar, local: 2, char: 'O'},
		{op: insertChar, local: 2, char: 'P'},
		{op: check, local: 2, str: "COOP"},
		// Copy l2 into l4
		{op: fork, local: 2, remote: 4},
		// Merge site #3 into #2
		{op: merge, local: 2, remote: 3},
		{op: check, local: 2, str: "COOP.IO"},
		// Merge site #4 (copy of #2 before merge) into #3
		{op: merge, local: 3, remote: 4},
		{op: check, local: 3, str: "COOP.IO"},
		// Ensure other streams are not changed.
		{op: check, local: 0, str: "C"},
		{op: check, local: 1, str: "CODE"},
	})
}

func TestUnknownRemoteYarn(t *testing.T) {
	teardown := setupUUIDs([]uuid.UUID{
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000002-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000003-8891-11ec-a04c-67855c00505b"),
	})
	defer teardown()

	// Site #0: A - B -----------------------.- *
	// Site #1:      `- C - D -------.- G - H'
	// Site #2:              `- E - F'
	runOperations(t, []operation{
		// Create site #0: AB
		{op: insertChar, local: 0, char: 'A'},
		{op: insertChar, local: 0, char: 'B'},
		// Create site #1 from #0: AB --> ABCD
		{op: fork, local: 0, remote: 1},
		{op: insertChar, local: 1, char: 'C'},
		{op: insertChar, local: 1, char: 'D'},
		{op: check, local: 1, str: "ABCD"},
		// Site #2: ABCD --> ABCDEF
		{op: fork, local: 1, remote: 2},
		{op: insertChar, local: 2, char: 'E'},
		{op: insertChar, local: 2, char: 'F'},
		{op: check, local: 2, str: "ABCDEF"},
		// Merge site #2 into #1: ABCDEF --> ABCDGHEF
		// Merging should not move the cursor (currently after D)
		{op: merge, local: 1, remote: 2},
		{op: insertChar, local: 1, char: 'G'},
		{op: insertChar, local: 1, char: 'H'},
		{op: check, local: 1, str: "ABCDGHEF"},
		// Merge site #1 into #0
		{op: merge, local: 0, remote: 1},
		{op: check, local: 0, str: "ABCDGHEF"},
	})
}
