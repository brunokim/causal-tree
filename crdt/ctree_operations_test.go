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

// Tests are structured as a sequence of operations on a list of trees.
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
// Trees are referred by their order of creation, NOT by their sitemap index.
// The fork operation requires specifying the correct remote index, even if it can be
// inferred from the number of already created trees, just to improve readability.
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
		return fmt.Sprintf("insert %c at tree #%d", op.char, op.local)
	case deleteChar:
		return fmt.Sprintf("delete char from tree #%d", op.local)
	case setCursor:
		return fmt.Sprintf("set cursor @ %d at tree #%d", op.pos, op.local)
	case insertCharAt:
		return fmt.Sprintf("insert %c @ %d at tree #%d", op.char, op.pos, op.local)
	case deleteCharAt:
		return fmt.Sprintf("delete char @ %d from tree #%d", op.pos, op.local)
	case fork:
		return fmt.Sprintf("fork tree #%d into tree #%d", op.local, op.remote)
	case merge:
		return fmt.Sprintf("merge tree #%d into tree #%d", op.remote, op.local)
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
func testOperations(t *testing.T, ops []operation) []*crdt.CausalTree {
	must := func(err error) {
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}
	trees := []*crdt.CausalTree{crdt.NewCausalTree()}
	f, err := setupTestFile(t.Name())
	if err != nil {
		t.Log(err)
	}
	for i, op := range ops {
		tree := trees[op.local]
		switch op.op {
		case insertChar:
			must(tree.InsertChar(op.char))
		case deleteChar:
			must(tree.DeleteChar())
		case setCursor:
			tree.SetCursor(op.pos)
		case insertStr:
			tree.InsertStr()
		case insertCharAt:
			must(tree.InsertCharAt(op.char, op.pos))
		case deleteCharAt:
			must(tree.DeleteCharAt(op.pos))
		case fork:
			if op.remote != len(trees) {
				t.Fatalf("fork: expecting remote index %d, got %d", op.remote, len(trees))
			}
			remote, err := tree.Fork()
			must(err)
			trees = append(trees, remote)
		case merge:
			tree.Merge(trees[op.remote])
		case check:
			if s := tree.ToString(); s != op.str {
				t.Errorf("%d: got tree[%d] = %q, want %q", i, op.local, s, op.str)
			}
		}
		// Dump trees into testfile.
		if f != nil && op.op != check {
			bs, err := json.Marshal(map[string]interface{}{
				"Type":   "test",
				"Action": op.String(),
				"Sites":  trees,
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
	return trees
}

// -----

// Execute list of operations, checking if they are well-formed.
func validateOperations(ops []operation) error {
	trees := []*crdt.CausalTree{crdt.NewCausalTree()}
	for _, op := range ops {
		if op.local >= len(trees) {
			return fmt.Errorf("invalid local index %d (len: %d), op: %v", op.local, len(trees), op)
		}
		tree := trees[op.local]
		switch op.op {
		case insertChar:
			if err := tree.InsertChar(op.char); err != nil {
				return fmt.Errorf("%v: %v", op, err)
			}
		case deleteChar:
			if err := tree.DeleteChar(); err != nil {
				return fmt.Errorf("%v: %v", op, err)
			}
		case setCursor:
			if err := tree.SetCursor(op.pos); err != nil {
				return fmt.Errorf("%v: %v", op, err)
			}
		case insertCharAt:
			if err := tree.InsertCharAt(op.char, op.pos); err != nil {
				return fmt.Errorf("%v: %v", op, err)
			}
		case deleteCharAt:
			if err := tree.DeleteCharAt(op.pos); err != nil {
				return fmt.Errorf("%v: %v", op, err)
			}
		case fork:
			if remote, err := tree.Fork(); err != nil {
				return fmt.Errorf("%v: %v", op, err)
			} else {
				trees = append(trees, remote)
			}
		case merge:
			if op.remote >= len(trees) {
				return fmt.Errorf("invalid remote index %d (len: %d), op: %v", op.remote, len(trees), op)
			} else {
				tree.Merge(trees[op.remote])
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

// Make a tree randomly, using some other sites to make it interesting.
func makeRandomTree(size int, r *rand.Rand) (*crdt.CausalTree, error) {
	const numLists = 10
	// Create trees forking from trees[0]
	trees := make([]*crdt.CausalTree, numLists)
	trees[0] = crdt.NewCausalTree()
	for i := 1; i < numLists; i++ {
		t, err := trees[0].Fork()
		if err != nil {
			return nil, err
		}
		trees[i] = t
	}
	n := 0
	for n < size {
		// Pick a random tree.
		i := r.Intn(numLists)
		t := trees[i]
		if i > 0 {
			t.Merge(trees[0])
		}
		// Insert or deletes 2-5 chars at tree.
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
				err = t.InsertCharAt(ch, pos)
				n++
			} else if p < 0.8 {
				err = t.InsertChar(ch)
				n++
			} else if p < 0.9 && n > 0 {
				pos := r.Intn(n)
				err = t.DeleteCharAt(pos)
				n--
			} else if t.Cursor != (crdt.AtomID{}) {
				err = t.DeleteChar()
				n--
			}
			if err != nil {
				return nil, err
			}
		}
		if i > 0 {
			trees[0].Merge(t)
		}
	}
	return trees[0], nil
}
