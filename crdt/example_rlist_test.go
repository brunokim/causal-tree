package crdt_test

import (
	"fmt"

	"github.com/brunokim/causal-tree/crdt"
)

// Showcasing the main operations in a replicated list (RList) data type.
func Example() {
	// Create new CRDT in l1, insert 'crdt is nice', and copy it to l2.
	l1 := crdt.NewRList()
	for _, ch := range "crdt is nice" {
		l1.InsertChar(ch)
	}
	l2, _ := l1.Fork()

	// Rewrite 'crdt is' with 'crdts are' in l2.
	l2.SetCursor(6) //             .-- place cursor here
	l2.DeleteChar() // c r d t _ i s
	l2.DeleteChar() //         ^ ^ ^
	l2.DeleteChar() //         and delete 3 chars
	for _, ch := range "s are" {
		l2.InsertChar(ch)
	}

	// Rewrite 'nice' with 'cool' in l1.
	for i := 0; i < 4; i++ {
		l1.DeleteChar()
	}
	for _, ch := range "cool" {
		l1.InsertChar(ch)
	}

	// Show contents of l1, l2, and then merge l2 into l1.
	fmt.Println("l1:", l1.AsString())
	fmt.Println("l2:", l2.AsString())
	l1.Merge(l2)
	fmt.Println("l1+l2:", l1.AsString())
	// Output:
	// l1: crdt is cool
	// l2: crdts are nice
	// l1+l2: crdts are cool
}

// Merging a set of overlapping changes may not produce intelligible results, but it's close
// enough to the intention of each party, and does not interrupt either to solve a merge conflict.
func ExampleRList_overlap() {
	l1 := crdt.NewRList()
	for _, ch := range "desserts" {
		l1.InsertChar(ch)
	}
	l2, _ := l1.Fork()

	// l1: desserts -> desert
	l1.DeleteCharAt(7)
	l1.DeleteCharAt(3)

	// l2: desserts -> dresser
	l2.DeleteCharAt(7)
	l2.DeleteCharAt(6)
	l2.InsertCharAt('r', 0)

	l1.Merge(l2)
	fmt.Println(l1.AsString())
	// Output: dreser
}
