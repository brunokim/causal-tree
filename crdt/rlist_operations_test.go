package crdt_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/brunokim/causal-tree/crdt"
)

// Tests are structured as a sequence of operations on a list of lists.
//
// This indirection allows us to perform some actions for every mutation, like
// dumping their internals to a file, and also allow us to fuzz list manipulation.
//
// Operations:
//
// insertChar <local> <char>         -- insert a char at cursor on list 'local'.
// deleteChar <local>                -- delete the char at cursor on list 'local'.
// setCursor <local> <pos>           -- set cursor at list-position 'pos' on list 'local'
// insertStr <local>                 -- insert a str container on list 'local'
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
	insertStr
)

var numBytes = map[operationType]int{
	insertChar:   3, // insertChar local char
	deleteChar:   2, // deleteChar local
	setCursor:    3, // setCursor local pos
	insertCharAt: 4, // insertCharAt local char pos
	deleteCharAt: 3, // deleteCharAt local pos
	fork:         2, // fork local
	merge:        3, // merge local remote
	//check is not encoded/decoded
}

type operation struct {
	op            operationType
	local, remote int
	char          rune
	pos           int
	str           string
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

func decodeOperation(bs []byte) (operation, int) {
	if len(bs) == 0 {
		return operation{}, 0
	}
	op := operationType(bs[0])
	n, ok := numBytes[op]
	if !ok || len(bs) < n {
		return operation{}, 0
	}
	toIndex := func(b byte) int { return int(b) }
	toChar := func(b byte) rune { return rune(b) + ' ' }
	toPos := func(b byte) int { return int(b) - 1 }
	result := operation{op: op, local: toIndex(bs[1])}
	switch op {
	case insertChar:
		result.char = toChar(bs[2])
	case deleteChar:
		// Do nothing
	case setCursor:
		result.pos = toPos(bs[2])
	case insertCharAt:
		result.char = toChar(bs[2])
		result.pos = toPos(bs[3])
	case deleteCharAt:
		result.pos = toPos(bs[2])
	case fork:
		// Do nothing
	case merge:
		result.remote = toIndex(bs[2])
	default:
		return operation{}, 0
	}
	return result, n
}

func decodeOperations(bs []byte) ([]operation, bool) {
	var ops []operation
	for len(bs) != 0 {
		op, n := decodeOperation(bs)
		if n == 0 {
			return nil, false
		}
		ops = append(ops, op)
		bs = bs[n:]
	}
	return ops, true
}

// -----

func setupTestFile(name string) (*os.File, error) {
	filename := fmt.Sprintf("testdata/%s.jsonl", name)
	baseDir := filepath.Dir(filename)
	os.MkdirAll(baseDir, 0777)
	return os.Create(filename)
}

// Execute sequence of operations dumping intermediate data structures into testdata.
func testOperations(t *testing.T, ops []operation) []*crdt.RList {
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
		case insertStr:
			list.InsertStr()
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
			if s := list.ToString(); s != op.str {
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

// Execute list of operations, checking if they are well-formed.
func validateOperations(ops []operation) error {
	lists := []*crdt.RList{crdt.NewRList()}
	for _, op := range ops {
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

func readFuzzData(filename string) ([]byte, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading fuzz corpus sample: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("expecting at least 2 lines in fuzz file %s", filename)
	}
	content := lines[1]
	if !(strings.HasPrefix(content, `[]byte("`) && strings.HasSuffix(content, `")`)) {
		return nil, fmt.Errorf(`expecting content enclosed by []byte("<content>"), got %s`, content)
	}
	start, end := len("[]byte("), len(content)-len(")")
	text, err := strconv.Unquote(content[start:end])
	if err != nil {
		return nil, fmt.Errorf("invalid syntax for fuzz corpus %s: %w", content, err)
	}
	bs, err := strconv.Unquote(`"` + text + `"`)
	if err != nil {
		return nil, fmt.Errorf("invalid syntax for byte slice %s: %w", text, err)
	}
	return []byte(bs), nil
}

// -----

// Make a list randomly, using some other sites to make it interesting.
func makeRandomList(size int, r *rand.Rand) (*crdt.RList, error) {
	const numLists = 10
	// Create lists forking from lists[0]
	lists := make([]*crdt.RList, numLists)
	lists[0] = crdt.NewRList()
	for i := 1; i < numLists; i++ {
		l, err := lists[0].Fork()
		if err != nil {
			return nil, err
		}
		lists[i] = l
	}
	n := 0
	for n < size {
		// Pick a random list.
		i := r.Intn(numLists)
		l := lists[i]
		if i > 0 {
			l.Merge(lists[0])
		}
		// Insert or deletes 2-5 chars at list.
		// 40%: inserts char at random.
		// 40%: inserts char at last position.
		// 10%: deletes char at random.
		// 10%: deletes char at last position.
		numEdits := 2 + r.Intn(4)
		for j := 0; j < numEdits; j++ {
			ch := rune(i) + 'a'
			var err error
			p := r.Float64()
			if p < 0.4 {
				pos := r.Intn(n+1) - 1 // pos in [-1,n)
				err = l.InsertCharAt(ch, pos)
				n++
			} else if p < 0.8 {
				err = l.InsertChar(ch)
				n++
			} else if p < 0.9 && n > 0 {
				pos := r.Intn(n)
				err = l.DeleteCharAt(pos)
				n--
			} else if l.Cursor != (crdt.AtomID{}) {
				err = l.DeleteChar()
				n--
			}
			if err != nil {
				return nil, err
			}
		}
		if i > 0 {
			lists[0].Merge(l)
		}
	}
	return lists[0], nil
}
