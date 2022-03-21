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

//
func ExampleRList_ViewAt() {
	s0 := crdt.NewRList()    // S0 @ T1
	s0.InsertChar('a')       // S0 @ T2
	s1, _ := s0.Fork()       // S0 @ T3
	s1.InsertChar('b')       // S1 @ T4
	s1.InsertChar('c')       // S1 @ T5
	s2, _ := s0.Fork()       // S0 @ T4
	s2.InsertCharAt('x', -1) // S2 @ T5
	s2.InsertChar('y')       // S2 @ T6
	s2.Merge(s1)             // S2 @ T7

	// Now s2 reads as "xyabc", with each char having the following IDs and causes:
	// x: s2 @ T5, caused by the zero atom
	// y: s2 @ T6, caused by s2 @ T5 (x)
	// a: s0 @ T2, caused by the zero atom
	// b: s1 @ T4, caused by s0 @ T2 (a)
	// c: s1 @ T5, caused by s1 @ T4 (b)

	// Timestamps for each site: s0=4, s1=5, s2=7
	v1, _ := s2.ViewAt(crdt.Weft{4, 5, 7})
	v2, _ := s2.ViewAt(crdt.Weft{4, 5, 0})
	v3, _ := s2.ViewAt(crdt.Weft{4, 0, 7})
	v4, _ := s2.ViewAt(crdt.Weft{0, 3, 7}) // With s0=0, we need to cut s1 down to T3, because it is ultimately caused by 'a' from s0.

	fmt.Println("Now: ", v1.AsString())
	fmt.Println("s2=0:", v2.AsString())
	fmt.Println("s1=0:", v3.AsString())
	fmt.Println("s0=0:", v4.AsString())
	// Output:
	// Now:  xyabc
	// s2=0: abc
	// s1=0: xya
	// s0=0: xy
}
