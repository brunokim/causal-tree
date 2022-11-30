package crdt_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/brunokim/causal-tree/crdt"
	"github.com/google/uuid"
)

func TestCausalTree(t *testing.T) {
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
	testOperations(t, []operation{
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
		{op: checkJSON, local: 0, str: `["C", "T", "R", "L"]`},
		// Site #1: CMD --> CMDALT
		{op: insertChar, local: 1, char: 'A'},
		{op: insertChar, local: 1, char: 'L'},
		{op: insertChar, local: 1, char: 'T'},
		{op: checkJSON, local: 1, str: `["C", "M", "D", "A", "L", "T"]`},
		// Site #2: CMD --> CMDDEL
		{op: insertChar, local: 2, char: 'D'},
		{op: insertChar, local: 2, char: 'E'},
		{op: insertChar, local: 2, char: 'L'},
		{op: checkJSON, local: 2, str: `["C", "M", "D", "D", "E", "L"]`},
		// Merge site #1 into #0 --> CTRLALT
		{op: merge, local: 0, remote: 1},
		{op: checkJSON, local: 0, str: `["C", "T", "R", "L", "A", "L", "T"]`},
		// Merge site #2 into #0 --> CTRLALTDEL
		{op: merge, local: 0, remote: 2},
		{op: checkJSON, local: 0, str: `["C", "T", "R", "L", "A", "L", "T", "D", "E", "L"]`},
		// Merge site #1 into #2 --> CMDALTDEL
		{op: merge, local: 2, remote: 1},
		{op: checkJSON, local: 2, str: `["C", "M", "D", "A", "L", "T", "D", "E", "L"]`},
		// Merge site #0 into #2 --> CTRLALTDEL
		{op: merge, local: 2, remote: 0},
		{op: checkJSON, local: 2, str: `["C", "T", "R", "L", "A", "L", "T", "D", "E", "L"]`},
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
	testOperations(t, []operation{
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
		{op: checkJSON, local: 3, str: `["C", "O", "D", "E", ".", "I", "O"]`},
		// Site #2: CODE --> COOP
		{op: deleteChar, local: 2},
		{op: deleteChar, local: 2},
		{op: insertChar, local: 2, char: 'O'},
		{op: insertChar, local: 2, char: 'P'},
		{op: checkJSON, local: 2, str: `["C", "O", "O", "P"]`},
		// Copy l2 into l4
		{op: fork, local: 2, remote: 4},
		// Merge site #3 into #2
		{op: merge, local: 2, remote: 3},
		{op: checkJSON, local: 2, str: `["C", "O", "O", "P", ".", "I", "O"]`},
		// Merge site #4 (copy of #2 before merge) into #3
		{op: merge, local: 3, remote: 4},
		{op: checkJSON, local: 3, str: `["C", "O", "O", "P", ".", "I", "O"]`},
		// Ensure other streams are not changed.
		{op: checkJSON, local: 0, str: `["C"]`},
		{op: checkJSON, local: 1, str: `["C", "O", "D", "E"]`},
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
	testOperations(t, []operation{
		// Create site #0: AB
		{op: insertChar, local: 0, char: 'A'},
		{op: insertChar, local: 0, char: 'B'},
		// Create site #1 from #0: AB --> ABCD
		{op: fork, local: 0, remote: 1},
		{op: insertChar, local: 1, char: 'C'},
		{op: insertChar, local: 1, char: 'D'},
		{op: checkJSON, local: 1, str: `["A", "B", "C", "D"]`},
		// Site #2: ABCD --> ABCDEF
		{op: fork, local: 1, remote: 2},
		{op: insertChar, local: 2, char: 'E'},
		{op: insertChar, local: 2, char: 'F'},
		{op: checkJSON, local: 2, str: `["A", "B", "C", "D", "E", "F"]`},
		// Merge site #2 into #1: ABCDEF --> ABCDGHEF
		// Merging should not move the cursor (currently after D)
		{op: merge, local: 1, remote: 2},
		{op: insertChar, local: 1, char: 'G'},
		{op: insertChar, local: 1, char: 'H'},
		{op: checkJSON, local: 1, str: `["A", "B", "C", "D", "G", "H", "E", "F"]`},
		// Merge site #1 into #0
		{op: merge, local: 0, remote: 1},
		{op: checkJSON, local: 0, str: `["A", "B", "C", "D", "G", "H", "E", "F"]`},
	})
}

func TestDeleteCursor(t *testing.T) {
	teardown := crdt.MockUUIDs(
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000002-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000003-8891-11ec-a04c-67855c00505b"),
	)
	defer teardown()

	testOperations(t, []operation{
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
		{op: checkJSON, local: 0, str: `["A", "X", "R", "S"]`},
	})
}

func TestSetCursor(t *testing.T) {
	teardown := crdt.MockUUIDs(
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
	)
	defer teardown()

	testOperations(t, []operation{
		// Create site #0: abcd
		{op: insertCharAt, char: 'a', pos: -1},
		{op: insertCharAt, char: 'b', pos: 0},
		{op: insertCharAt, char: 'c', pos: 1},
		{op: insertCharAt, char: 'd', pos: 2},
		// Transform abcd -> xabdy
		{op: insertCharAt, char: 'x', pos: -1},
		{op: checkJSON, str: `["x", "a", "b", "c", "d"]`},
		{op: deleteCharAt, pos: 3},
		{op: checkJSON, str: `["x", "a", "b", "d"]`},
		{op: insertCharAt, char: 'y', pos: 3},
		{op: checkJSON, str: `["x", "a", "b", "d", "y"]`},
	})
}

func TestDeleteAfterMerge(t *testing.T) {
	teardown := crdt.MockUUIDs(
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000002-8891-11ec-a04c-67855c00505b"),
	)
	defer teardown()

	testOperations(t, []operation{
		// Create site #0: abcd
		{op: insertChar, local: 0, char: 'a'},
		{op: insertChar, local: 0, char: 'b'},
		{op: insertChar, local: 0, char: 'c'},
		{op: insertChar, local: 0, char: 'd'},
		{op: checkJSON, local: 0, str: `["a", "b", "c", "d"]`},
		// Create site #1: abcd -> xabdy
		{op: fork, local: 0, remote: 1},
		{op: insertCharAt, local: 1, char: 'x', pos: -1},
		{op: deleteCharAt, local: 1, pos: 3},
		{op: insertCharAt, local: 1, char: 'y', pos: 3},
		{op: setCursor, local: 1, pos: 4},
		{op: checkJSON, local: 1, str: `["x", "a", "b", "d", "y"]`},
		// Edit site #0: abcd -> abcdefg
		{op: insertChar, local: 0, char: 'e'},
		{op: insertChar, local: 0, char: 'f'},
		{op: insertChar, local: 0, char: 'g'},
		{op: checkJSON, local: 0, str: `["a", "b", "c", "d", "e", "f", "g"]`},
		// Merge site #0 -> site #1
		{op: merge, local: 1, remote: 0},
		{op: checkJSON, local: 1, str: `["x", "a", "b", "d", "y", "e", "f", "g"]`},
		// Merge site #1 -> site #0
		{op: merge, local: 0, remote: 1},
		{op: checkJSON, local: 0, str: `["x", "a", "b", "d", "y", "e", "f", "g"]`},
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
		{op: checkJSON, local: 0, str: `["E"]`},
	})
}

func TestInsertsAtSamePosition(t *testing.T) {
	teardown := crdt.MockUUIDs(
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
	)
	defer teardown()

	testOperations(t, []operation{
		// Create site inserting all letters at the same position.
		{op: insertCharAt, char: 'd', pos: -1},
		{op: insertCharAt, char: 'e', pos: -1},
		{op: insertCharAt, char: 's', pos: -1},
		{op: insertCharAt, char: 's', pos: -1},
		{op: insertCharAt, char: 'e', pos: -1},
		{op: insertCharAt, char: 'r', pos: -1},
		{op: insertCharAt, char: 't', pos: -1},
		{op: insertCharAt, char: 's', pos: -1},
		{op: checkJSON, str: `["s", "t", "r", "e", "s", "s", "e", "d"]`},
	})
}

// -----

func setupTestView(t *testing.T) []*crdt.CausalTree {
	return testOperations(t, []operation{
		// Create site #0: abcd
		{op: insertChar, local: 0, char: 'a'},
		{op: insertChar, local: 0, char: 'b'},
		{op: insertChar, local: 0, char: 'c'},
		{op: insertChar, local: 0, char: 'd'},
		{op: checkJSON, local: 0, str: `["a", "b", "c", "d"]`},
		// Create site #1: abcd -> xabdy
		{op: fork, local: 0, remote: 1},
		{op: insertCharAt, local: 1, char: 'x', pos: -1},
		{op: deleteCharAt, local: 1, pos: 3},
		{op: insertCharAt, local: 1, char: 'y', pos: 3},
		{op: checkJSON, local: 1, str: `["x", "a", "b", "d", "y"]`},
		// Edit site #0: abcde -> abcdefg
		{op: insertChar, local: 0, char: 'e'},
		{op: insertChar, local: 0, char: 'f'},
		{op: insertChar, local: 0, char: 'g'},
		{op: checkJSON, local: 0, str: `["a", "b", "c", "d", "e", "f", "g"]`},
		// Merge site #1 -> site #0
		{op: merge, local: 0, remote: 1},
		{op: checkJSON, local: 0, str: `["x", "a", "b", "d", "y", "e", "f", "g"]`},
	})
	// Now, max time is [9 9] for both sites.
}

func TestViewAt(t *testing.T) {
	teardown := crdt.MockUUIDs(
		uuid.MustParse("00000001-8891-11ec-a04c-67855c00505b"),
		uuid.MustParse("00000002-8891-11ec-a04c-67855c00505b"),
	)
	defer teardown()

	trees := setupTestView(t)
	t0 := trees[0]

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
		view, err := t0.ViewAt(test.weft)
		if err != nil {
			t.Fatalf("%v: got err, want nil: %v", test.weft, err)
		}
		got := view.ToString()
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

	trees := setupTestView(t)
	t0 := trees[0]

	tests := []struct {
		weft crdt.Weft
	}{
		// At timestamp 5 at site #0 we cut 'd', whose child 'y' in site #1 is at timestamp 9.
		{crdt.Weft{4, 9}}, {crdt.Weft{3, 9}}, {crdt.Weft{2, 9}}, {crdt.Weft{1, 9}},
		// At timestamp 4 at site #0 we cut 'c', whose delete child in site #1 is at timestamp 8.
		{crdt.Weft{3, 8}}, {crdt.Weft{2, 8}}, {crdt.Weft{1, 8}},
	}
	for _, test := range tests {
		view, err := t0.ViewAt(test.weft)
		if err == nil {
			t.Fatalf("%v: got nil, want err (str: %q)", test.weft, view.ToString())
		}
	}
}

//Tests for insertStr

/*Edge cases*/
func TestInsertStrEdgeCases(t *testing.T) {

	t.Run("EmptyStrings", func(t *testing.T) {
		testOperations(t, []operation{
			// Insert empty str
			{op: insertStr, local: 0},
			{op: checkJSON, local: 0, str: `[""]`},
			// Insert another empty str
			{op: insertStr, local: 0},
			{op: checkJSON, local: 0, str: `["", ""]`},
		})
	})
	t.Run("MergeStr", func(t *testing.T) {
		testOperations(t, []operation{
			// Fork site 0:
			{op: fork, local: 0, remote: 1},
			// Insert str 'a' into site 0
			{op: insertStr, local: 0},
			{op: insertChar, local: 0, char: 'a'},
			{op: checkJSON, local: 0, str: `["a"]`},
			// Insert str 'b' into site 1
			{op: insertStr, local: 1},
			{op: insertChar, local: 1, char: 'b'},
			{op: checkJSON, local: 1, str: `["b"]`},
			// Merge site #1 -> site #0
			{op: merge, local: 0, remote: 1},
			{op: checkJSON, local: 0, str: `["a", "b"]`},
		})
	})
	t.Run("DeleteEmptyStr", func(t *testing.T) {
		testOperations(t, []operation{
			// Delete str container:
			{op: insertStr, local: 0},
			{op: deleteCharAt, pos: 0},
			{op: checkJSON, local: 0, str: `null`},
		})
	})
	t.Run("DeleteNonEmptyStr", func(t *testing.T) {
		testOperations(t, []operation{
			// Insert str1 -> 'a' and delete the str container:
			{op: insertStr, local: 0},
			{op: insertChar, local: 0, char: 'a'},
			{op: deleteCharAt, pos: 0},
			{op: checkJSON, local: 0, str: `null`},
		})
	})
}

/*--------*/

func TestInsertStrDomainCases(t *testing.T) {
	t.Run("Insert2Strings", func(t *testing.T) {
		testOperations(t, []operation{
			// Create site #0: str1->bcd
			{op: insertStr, local: 0},
			{op: insertChar, local: 0, char: 'b'},
			{op: insertChar, local: 0, char: 'c'},
			{op: insertChar, local: 0, char: 'd'},
			{op: checkJSON, local: 0, str: `["bcd"]`},
			// Insert another str container: str2 -> efg, str1 -> bcd
			{op: insertStr, local: 0},
			{op: insertChar, local: 0, char: 'e'},
			{op: insertChar, local: 0, char: 'f'},
			{op: insertChar, local: 0, char: 'g'},
			{op: checkJSON, local: 0, str: `["efg", "bcd"]`},
		})
	})

	t.Run("Insert2StringsAndDeleteTheFirst", func(t *testing.T) {
		testOperations(t, []operation{
			// Create site #0: str1->bcd
			{op: insertStr, local: 0},
			{op: insertChar, local: 0, char: 'b'},
			{op: insertChar, local: 0, char: 'c'},
			{op: insertChar, local: 0, char: 'd'},
			{op: checkJSON, local: 0, str: `["bcd"]`},
			// Insert another str container: str2 -> efg, str1 -> bcd
			{op: insertStr, local: 0},
			{op: insertChar, local: 0, char: 'e'},
			{op: insertChar, local: 0, char: 'f'},
			{op: insertChar, local: 0, char: 'g'},
			{op: checkJSON, local: 0, str: `["efg", "bcd"]`},
			//Delete the string 'efg'
			{op: deleteCharAt, pos: 0},
			{op: checkJSON, local: 0, str: `["bcd"]`},
		})
	})
	t.Run("Insert2StringsAndDeleteTheSecond", func(t *testing.T) {
		testOperations(t, []operation{
			// Create site #0: str1->bcd
			{op: insertStr, local: 0},
			{op: insertChar, local: 0, char: 'b'},
			{op: insertChar, local: 0, char: 'c'},
			{op: insertChar, local: 0, char: 'd'},
			{op: checkJSON, local: 0, str: `["bcd"]`},
			// Insert another str container: str2 -> efg, str1 -> bcd
			{op: insertStr, local: 0},
			{op: insertChar, local: 0, char: 'e'},
			{op: insertChar, local: 0, char: 'f'},
			{op: insertChar, local: 0, char: 'g'},
			{op: checkJSON, local: 0, str: `["efg", "bcd"]`},
			//Delete the string 'efg'
			{op: deleteCharAt, pos: 4},
			{op: checkJSON, local: 0, str: `["efg"]`},
		})
	})

}

func TestMergeMultipleStrContainers(t *testing.T) {
	testOperations(t, []operation{
		// Fork site 0:
		{op: fork, local: 0, remote: 1},
		// Create site #0: str1->bcd
		{op: insertStr, local: 0},
		{op: insertChar, local: 0, char: 'b'},
		{op: insertChar, local: 0, char: 'c'},
		{op: insertChar, local: 0, char: 'd'},
		{op: checkJSON, local: 0, str: `["bcd"]`},
		// fork and
		{op: insertStr, local: 1},
		{op: insertChar, local: 1, char: 'e'},
		{op: insertChar, local: 1, char: 'f'},
		{op: insertChar, local: 1, char: 'g'},
		{op: checkJSON, local: 1, str: `["efg"]`},
		// Merge site #1 -> site #0
		{op: merge, local: 0, remote: 1},
		{op: checkJSON, local: 0, str: `["bcd", "efg"]`},
	})
}

// -----

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
		data, err := readFuzzData(filepath.Join("testdata/fuzz/FuzzList", file.Name()))
		if err != nil {
			t.Fatalf("reading file %s failed: %v", file.Name(), err)
		}
		ops, ok := decodeOperations(data)
		if !ok {
			t.Fatalf("can't decode data")
		}
		if err := validateOperations(ops); err != nil {
			t.Errorf("execution of file %s failed: %v", file.Name(), err)
		}
	}
}

func FuzzList(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		if ops, ok := decodeOperations(data); ok {
			validateOperations(ops)
		}
	})
}

func FuzzViewAt(f *testing.F) {
	tree, err := makeRandomTree(200, newRand())
	if err != nil {
		f.Fatalf("error making tree: %v", err)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		weft := tree.Now()
		for i, x := range data {
			if i >= len(weft) {
				break
			}
			weft[i] = uint32(x)
		}
		tree.ViewAt(weft)
	})
}

// -----

func newRand() *rand.Rand {
	return rand.New(rand.NewSource(1740))
}

var (
	sizes      = []int{64, 256, 1024, 4096, 16384}
	benchTrees = map[int]*crdt.CausalTree{}
)

func getBenchTree(size int) *crdt.CausalTree {
	tree, ok := benchTrees[size]
	if !ok {
		tree, _ = makeRandomTree(size, newRand())
		benchTrees[size] = tree
	}
	return tree
}

func BenchmarkFork(b *testing.B) {
	for _, size := range sizes {
		tree := getBenchTree(size)
		name := fmt.Sprintf("size=%d", size)
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				t1 := tree.Clone()
				if _, err := t1.Fork(); err != nil {
					b.Error(err)
				}
			}
		})
	}
}

func BenchmarkSetCursor(b *testing.B) {
	for _, size := range sizes {
		tree := getBenchTree(size)
		name := fmt.Sprintf("size=%d", size)
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				tree.SetCursor(size / 2)
			}
		})
	}
}

func BenchmarkInsertChar(b *testing.B) {
	for _, size := range sizes {
		tree := getBenchTree(size)
		name := fmt.Sprintf("size=%d", size)
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				t1 := tree.Clone()
				t1.SetCursor(size / 2)
				if err := t1.InsertChar('x'); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkDeleteChar(b *testing.B) {
	for _, size := range sizes {
		tree := getBenchTree(size)
		name := fmt.Sprintf("size=%d", size)
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				t1 := tree.Clone()
				t1.SetCursor(size / 2)
				if err := t1.DeleteChar(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkMerge(b *testing.B) {
	for _, size := range sizes {
		r := newRand()
		r.Seed(5461)
		remote, _ := makeRandomTree(size, r)
		tree := getBenchTree(size)
		name := fmt.Sprintf("size=%d", size)
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				t1 := tree.Clone()
				t1.Merge(remote)
			}
		})
	}
}
