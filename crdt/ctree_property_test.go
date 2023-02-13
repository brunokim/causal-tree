package crdt_test

import (
	"testing"

	"github.com/brunokim/causal-tree/crdt"
	"pgregory.net/rapid"
)

// Model an CausalTree as a slice of chars, subject to insertions and deletions
// at random positions with InsertCharAt and DeleteCharAt.
//
// We don't model the more primitive operations InsertChar, DeleteChar and SetCursor
// because it's complicated to model where the cursor ends up after a deletion. The
// cursor moves to the deleted atom's cause, or its first non-deleted ancestor,
// which may not be the char next to it.
//
// TODO: perhaps this is a sign that the cursor should be more predictable...?
type stateMachine struct {
	t     *crdt.CausalTree
	chars []rune
}

func (m *stateMachine) Init(t *rapid.T) {
	m.t = crdt.NewCausalTree()
}

func (m *stateMachine) InsertCharAt(t *rapid.T) {
	ch := rapid.Rune().Draw(t, "ch").(rune)
	i := rapid.IntRange(-1, len(m.chars)-1).Draw(t, "i").(int)

	err := m.t.InsertCharAt(ch, i)
	if err != nil {
		t.Fatal("(*stateMachine).InsertCharAt:", err)
	}

	m.chars = append(m.chars[:i+1], append([]rune{ch}, m.chars[i+1:]...)...)
}

func (m *stateMachine) DeleteCharAt(t *rapid.T) {
	if len(m.chars) == 0 {
		t.Skip("empty string")
	}
	i := rapid.IntRange(0, len(m.chars)-1).Draw(t, "i").(int)

	err := m.t.DeleteAt(i)
	if err != nil {
		t.Fatal("(*stateMachine).DeleteCharAt:", err)
	}

	copy(m.chars[i:], m.chars[i+1:])
	m.chars = m.chars[:len(m.chars)-1]
}

func (m *stateMachine) Check(t *rapid.T) {
	got := m.t.ToString()
	want := string(m.chars)
	if got != want {
		t.Fatalf("content mismatch: want %q but got %q", want, got)
	}
	t.Log("content:", got)
}

func TestProperty(t *testing.T) {
	rapid.Check(t, rapid.Run(&stateMachine{}))
}
