package crdt_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/brunokim/causal-tree/crdt"
	"github.com/google/uuid"
)

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
// insertChar <local> <char>         -- insert a char at cursor on list 'local'.
// deleteChar <local>                -- delete the char at cursor on list 'local'.
// setCursor <local> <pos>           -- set cursor at list-position 'pos' on list 'local'
// insertCharAt <local> <char> <pos> -- insert a char at list-position 'pos' on list 'local'
// deleteCharAt <local> <pos>        -- delete char at list-position 'pos' on list 'local'
// fork <local> <remote>             -- fork list 'local' into list 'remote'.
// merge <local> <remote>            -- merge list 'remote' into list 'local'.
// check <local> <str>               -- check that the contents of 'local' spell 'str'.
//
// Lists are referred by their order of creation, NOT by their sitemap index.
// The fork operation requires specifying the correct remote index, even if it can be
// inferred from the number of already created lists, just to improve readability.
// 'list-position' refers to the position in the *resulting* list, not in the weave.

type operationType int

const (
	insertChar operationType = iota
	deleteChar
	setCursor
	insertCharAt
	deleteCharAt
	fork
	merge
	check
)

var (
	insertCharRE   = regexp.MustCompile(`^insertChar (\d+) (.)$`)
	deleteCharRE   = regexp.MustCompile(`^deleteChar (\d+)$`)
	setCursorRE    = regexp.MustCompile(`^setCursor (\d+) (-1|\d+)$`)
	insertCharAtRE = regexp.MustCompile(`^insertCharAt (\d+) (.) (-1|\d+)$`)
	deleteCharAtRE = regexp.MustCompile(`^deleteCharAt (\d+) (\d+)$`)
	forkRE         = regexp.MustCompile(`^fork (\d+) (\d+)$`)
	mergeRE        = regexp.MustCompile(`^merge (\d+) (\d+)$`)
)

type operation struct {
	op            operationType
	local, remote int
	char          rune
	pos           int
	str           string
}

func parseInt(text string) (int, bool) {
	i, err := strconv.Atoi(text)
	return i, (err == nil)
}

func parseChar(text string) rune {
	return []rune(text)[0]
}

func parseOperation(text string) (operation, bool) {
	if parts := insertCharRE.FindStringSubmatch(text); parts != nil {
		local, ok := parseInt(parts[1])
		char := parseChar(parts[2])
		return operation{op: insertChar, local: local, char: char}, ok
	}
	if parts := deleteCharRE.FindStringSubmatch(text); parts != nil {
		local, ok := parseInt(parts[1])
		return operation{op: deleteChar, local: local}, ok
	}
	if parts := setCursorRE.FindStringSubmatch(text); parts != nil {
		local, ok1 := parseInt(parts[1])
		pos, ok2 := parseInt(parts[2])
		return operation{op: setCursor, local: local, pos: pos}, ok1 && ok2
	}
	if parts := insertCharAtRE.FindStringSubmatch(text); parts != nil {
		local, ok1 := parseInt(parts[1])
		char := parseChar(parts[2])
		pos, ok2 := parseInt(parts[3])
		return operation{op: insertChar, local: local, char: char, pos: pos}, ok1 && ok2
	}
	if parts := deleteCharAtRE.FindStringSubmatch(text); parts != nil {
		local, ok1 := parseInt(parts[1])
		pos, ok2 := parseInt(parts[2])
		return operation{op: insertChar, local: local, pos: pos}, ok1 && ok2
	}
	if parts := forkRE.FindStringSubmatch(text); parts != nil {
		local, ok1 := parseInt(parts[1])
		remote, ok2 := parseInt(parts[2])
		return operation{op: fork, local: local, remote: remote}, ok1 && ok2
	}
	if parts := mergeRE.FindStringSubmatch(text); parts != nil {
		local, ok1 := parseInt(parts[1])
		remote, ok2 := parseInt(parts[2])
		return operation{op: merge, local: local, remote: remote}, ok1 && ok2
	}
	return operation{}, false
}

func (op operation) String() string {
	switch op.op {
	case insertChar:
		return fmt.Sprintf("insert %c at list #%d", op.char, op.local)
	case deleteChar:
		return fmt.Sprintf("delete char from list #%d", op.local)
	case setCursor:
		return fmt.Sprintf("set cursor @ %d at list #%d", op.pos, op.local)
	case insertCharAt:
		return fmt.Sprintf("insert %c @ %d at list #%d", op.char, op.pos, op.local)
	case deleteCharAt:
		return fmt.Sprintf("delete char @ %d from list #%d", op.pos, op.local)
	case fork:
		return fmt.Sprintf("fork list #%d into list #%d", op.local, op.remote)
	case merge:
		return fmt.Sprintf("merge list #%d into list #%d", op.remote, op.local)
	}
	return ""
}

func runOperations(t *testing.T, ops []operation) []*crdt.RList {
	must := func(err error) {
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}
	lists := []*crdt.RList{crdt.NewRList()}
	f, err := setupTestFile(t.Name())
	if err != nil {
		t.Log(err)
	}
	for i, op := range ops {
		list := lists[op.local]
		switch op.op {
		case insertChar:
			must(list.InsertChar(op.char))
		case deleteChar:
			must(list.DeleteChar())
		case setCursor:
			list.SetCursor(op.pos)
		case insertCharAt:
			must(list.InsertCharAt(op.char, op.pos))
		case deleteCharAt:
			must(list.DeleteCharAt(op.pos))
		case fork:
			if op.remote != len(lists) {
				t.Fatalf("fork: expecting remote index %d, got %d", op.remote, len(lists))
			}
			remote, err := list.Fork()
			must(err)
			lists = append(lists, remote)
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
				"Type":   "test",
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
	teardown := crdt.MockUUIDs(
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000002-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000003-8891-11ec-a04c-67855c00505b"),
	)
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
	teardown := crdt.MockUUIDs(
		// UUIDs don't progress with increasing timestamp: 1,5,2,4,3
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000005-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000002-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000004-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000003-8891-11ec-a04c-67855c00505b"),
	)
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
	teardown := crdt.MockUUIDs(
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000002-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000003-8891-11ec-a04c-67855c00505b"),
	)
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

func TestDeleteCursor(t *testing.T) {
	teardown := crdt.MockUUIDs(
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000002-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000003-8891-11ec-a04c-67855c00505b"),
	)
	defer teardown()

	runOperations(t, []operation{
		// Create site #0: AB
		{op: insertChar, local: 0, char: 'A'},
		{op: insertChar, local: 0, char: 'B'},
		// Create site #1 from #0: AB --> ABC
		{op: fork, local: 0, remote: 1},
		{op: insertChar, local: 1, char: 'C'},
		// Merge site #1 into #0: cursor still at B
		{op: merge, local: 0, remote: 1},
		// Create site #2 from #1: ABC --> ARS
		{op: fork, local: 1, remote: 2},
		{op: deleteChar, local: 2},
		{op: deleteChar, local: 2},
		{op: insertChar, local: 2, char: 'R'},
		{op: insertChar, local: 2, char: 'S'},
		// Merge site #2 into #0: B is deleted and cursor is updated to A.
		{op: merge, local: 0, remote: 2},
		{op: insertChar, local: 0, char: 'X'},
		{op: check, local: 0, str: "AXRS"},
	})
}

func TestSetCursor(t *testing.T) {
	teardown := crdt.MockUUIDs(
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
	)
	defer teardown()

	runOperations(t, []operation{
		// Create site #0: abcd
		{op: insertCharAt, char: 'a', pos: -1},
		{op: insertCharAt, char: 'b', pos: 0},
		{op: insertCharAt, char: 'c', pos: 1},
		{op: insertCharAt, char: 'd', pos: 2},
		// Transform abcd -> xabdy
		{op: insertCharAt, char: 'x', pos: -1},
		{op: check, str: "xabcd"},
		{op: deleteCharAt, pos: 3},
		{op: check, str: "xabd"},
		{op: insertCharAt, char: 'y', pos: 3},
		{op: check, str: "xabdy"},
	})
}

func TestDeleteAfterMerge(t *testing.T) {
	teardown := crdt.MockUUIDs(
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000002-8891-11ec-a04c-67855c00505b"),
	)
	defer teardown()

	runOperations(t, []operation{
		// Create site #0: abcd
		{op: insertChar, local: 0, char: 'a'},
		{op: insertChar, local: 0, char: 'b'},
		{op: insertChar, local: 0, char: 'c'},
		{op: insertChar, local: 0, char: 'd'},
		{op: check, local: 0, str: "abcd"},
		// Create site #1: abcd -> xabdy
		{op: fork, local: 0, remote: 1},
		{op: insertCharAt, local: 1, char: 'x', pos: -1},
		{op: deleteCharAt, local: 1, pos: 3},
		{op: insertCharAt, local: 1, char: 'y', pos: 3},
		{op: setCursor, local: 1, pos: 4},
		{op: check, local: 1, str: "xabdy"},
		// Edit site #0: abcd -> abcdefg
		{op: insertChar, local: 0, char: 'e'},
		{op: insertChar, local: 0, char: 'f'},
		{op: insertChar, local: 0, char: 'g'},
		{op: check, local: 0, str: "abcdefg"},
		// Merge site #0 -> site #1
		{op: merge, local: 1, remote: 0},
		{op: check, local: 1, str: "xabdyefg"},
		// Merge site #1 -> site #0
		{op: merge, local: 0, remote: 1},
		{op: check, local: 0, str: "xabdyefg"},
		// Delete everything from site #0: xabdyefg -> E
		{op: insertCharAt, local: 0, char: 'E', pos: -1},
		{op: deleteCharAt, local: 0, pos: 1}, // x
		{op: deleteCharAt, local: 0, pos: 1}, // a
		{op: deleteCharAt, local: 0, pos: 1}, // b
		{op: deleteCharAt, local: 0, pos: 1}, // d
		{op: deleteCharAt, local: 0, pos: 1}, // y
		{op: deleteCharAt, local: 0, pos: 1}, // e
		{op: deleteCharAt, local: 0, pos: 1}, // f
		{op: deleteCharAt, local: 0, pos: 1}, // g
		{op: check, local: 0, str: "E"},
	})
}

func TestInsertsAtSamePosition(t *testing.T) {
	teardown := crdt.MockUUIDs(
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
	)
	defer teardown()

	runOperations(t, []operation{
		// Create site inserting all letters at the same position.
		{op: insertCharAt, char: 'd', pos: -1},
		{op: insertCharAt, char: 'e', pos: -1},
		{op: insertCharAt, char: 's', pos: -1},
		{op: insertCharAt, char: 's', pos: -1},
		{op: insertCharAt, char: 'e', pos: -1},
		{op: insertCharAt, char: 'r', pos: -1},
		{op: insertCharAt, char: 't', pos: -1},
		{op: insertCharAt, char: 's', pos: -1},
		{op: check, str: "stressed"},
	})
}

// -----

func setupTestView(t *testing.T) []*crdt.RList {
	return runOperations(t, []operation{
		// Create site #0: abcd
		{op: insertChar, local: 0, char: 'a'},
		{op: insertChar, local: 0, char: 'b'},
		{op: insertChar, local: 0, char: 'c'},
		{op: insertChar, local: 0, char: 'd'},
		{op: check, local: 0, str: "abcd"},
		// Create site #1: abcd -> xabdy
		{op: fork, local: 0, remote: 1},
		{op: insertCharAt, local: 1, char: 'x', pos: -1},
		{op: deleteCharAt, local: 1, pos: 3},
		{op: insertCharAt, local: 1, char: 'y', pos: 3},
		{op: check, local: 1, str: "xabdy"},
		// Edit site #0: abcde -> abcdefg
		{op: insertChar, local: 0, char: 'e'},
		{op: insertChar, local: 0, char: 'f'},
		{op: insertChar, local: 0, char: 'g'},
		{op: check, local: 0, str: "abcdefg"},
		// Merge site #1 -> site #0
		{op: merge, local: 0, remote: 1},
		{op: check, local: 0, str: "xabdyefg"},
	})
	// Now, max time is [9 9] for both sites.
}

func TestViewAt(t *testing.T) {
	teardown := crdt.MockUUIDs(
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000002-8891-11ec-a04c-67855c00505b"),
	)
	defer teardown()

	lists := setupTestView(t)
	l0 := lists[0]

	tests := []struct {
		weft crdt.Weft
		want string
	}{
		// Now
		{crdt.Weft{9, 9}, "xabdyefg"},
		// Far future
		{crdt.Weft{100, 100}, "xabdyefg"},
		// "Undoing" actions of site #0
		{crdt.Weft{8, 9}, "xabdyef"},
		{crdt.Weft{7, 9}, "xabdye"},
		{crdt.Weft{6, 9}, "xabdy"},
		{crdt.Weft{5, 9}, "xabdy"},
		{crdt.Weft{4, 8}, "xab"}, // Cutting 'd' from site #0 requires removing its child 'y' from site #1
		{crdt.Weft{3, 7}, "xab"}, // Cutting 'c' from site #0 requires removing its delete child from site #1
		{crdt.Weft{2, 7}, "xa"},
		{crdt.Weft{1, 7}, "x"},
		{crdt.Weft{0, 7}, "x"},
		// "Undoing" actions of site #1
		{crdt.Weft{9, 8}, "xabdefg"},
		{crdt.Weft{9, 7}, "xabcdefg"},
		{crdt.Weft{9, 6}, "abcdefg"},
		{crdt.Weft{9, 5}, "abcdefg"},
		{crdt.Weft{9, 4}, "abcdefg"},
		{crdt.Weft{9, 3}, "abcdefg"},
		{crdt.Weft{9, 2}, "abcdefg"},
		{crdt.Weft{9, 1}, "abcdefg"},
		{crdt.Weft{9, 0}, "abcdefg"},
	}
	for _, test := range tests {
		view, err := l0.ViewAt(test.weft)
		if err != nil {
			t.Fatalf("%v: got err, want nil: %v", test.weft, err)
		}
		got := view.AsString()
		if got != test.want {
			t.Errorf("%v: got %q, want %q", test.weft, got, test.want)
		}
	}
}

func TestViewAtError(t *testing.T) {
	teardown := crdt.MockUUIDs(
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000002-8891-11ec-a04c-67855c00505b"),
	)
	defer teardown()

	lists := setupTestView(t)
	l0 := lists[0]

	tests := []struct {
		weft crdt.Weft
	}{
		// At timestamp 5 at site #0 we cut 'd', whose child 'y' in site #1 is at timestamp 9.
		{crdt.Weft{4, 9}}, {crdt.Weft{3, 9}}, {crdt.Weft{2, 9}}, {crdt.Weft{1, 9}},
		// At timestamp 4 at site #0 we cut 'c', whose delete child in site #1 is at timestamp 8.
		{crdt.Weft{3, 8}}, {crdt.Weft{2, 8}}, {crdt.Weft{1, 8}},
	}
	for _, test := range tests {
		view, err := l0.ViewAt(test.weft)
		if err == nil {
			t.Fatalf("%v: got nil, want err (str: %q)", test.weft, view.AsString())
		}
	}
}

// -----

func validateOperations(text string) error {
	lists := []*crdt.RList{crdt.NewRList()}
	for _, opStr := range strings.Split(text, ",") {
		op, ok := parseOperation(opStr)
		if !ok {
			return fmt.Errorf("parse error at %v", op)
		}
		if op.local >= len(lists) {
			return fmt.Errorf("invalid local index %d (len: %d), op: %v", op.local, len(lists), op)
		}
		list := lists[op.local]
		switch op.op {
		case insertChar:
			if err := list.InsertChar(op.char); err != nil {
				return fmt.Errorf("%v: %v", op, err)
			}
		case deleteChar:
			if err := list.DeleteChar(); err != nil {
				return fmt.Errorf("%v: %v", op, err)
			}
		case setCursor:
			if err := list.SetCursor(op.pos); err != nil {
				return fmt.Errorf("%v: %v", op, err)
			}
		case insertCharAt:
			if err := list.InsertCharAt(op.char, op.pos); err != nil {
				return fmt.Errorf("%v: %v", op, err)
			}
		case deleteCharAt:
			if err := list.DeleteCharAt(op.pos); err != nil {
				return fmt.Errorf("%v: %v", op, err)
			}
		case fork:
			if remote, err := list.Fork(); err != nil {
				return fmt.Errorf("%v: %v", op, err)
			} else {
				lists = append(lists, remote)
			}
		case merge:
			if op.remote >= len(lists) {
				return fmt.Errorf("invalid remote index %d (len: %d), op: %v", op.remote, len(lists), op)
			} else {
				list.Merge(lists[op.remote])
			}
		default:
			return fmt.Errorf("invalid op %v", op.op)
		}
	}
	return nil
}

func readFuzzText(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("error reading fuzz corpus sample: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("expecting at least 2 lines in fuzz file %s", filename)
	}
	content := lines[1]
	if !(strings.HasPrefix(content, `string("`) && strings.HasSuffix(content, `")`)) {
		return "", fmt.Errorf(`expecting content enclosed by string("<content>"), got %s`, content)
	}
	start, end := len("string("), len(content)-len(")")
	text, err := strconv.Unquote(content[start:end])
	if err != nil {
		return "", fmt.Errorf("invalid syntax for fuzz corpus %s: %w", content, err)
	}
	return text, nil
}

func TestValidateFuzzList(t *testing.T) {
	f, err := os.Open("testdata/fuzz/FuzzList")
	defer f.Close()
	if err != nil {
		t.Fatalf("error opening fuzz corpus directory: %v", err)
	}
	files, err := f.ReadDir(-1)
	if err != nil {
		t.Fatalf("error listing fuzz corpus: %v", err)
	}
	for _, file := range files {
		text, err := readFuzzText(filepath.Join("testdata/fuzz/FuzzList", file.Name()))
		if err != nil {
			t.Fatalf("reading file %s failed: %v", file.Name(), err)
		}
		if err := validateOperations(text); err != nil {
			t.Errorf("execution of file %s failed: %v", file.Name(), err)
		}
	}
}

func FuzzList(f *testing.F) {
	f.Fuzz(func(t *testing.T, data string) {
		validateOperations(data)
	})
}
