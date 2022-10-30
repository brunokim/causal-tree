package crdt_test

import (
	"fmt"

	"github.com/brunokim/causal-tree/crdt"
)

// Showcasing the main operations in a replicated list (CausalTree) data type.
func Example() {
	// Create new CRDT in t1, insert 'crdt is nice', and copy it to t2.
	t1 := crdt.NewCausalTree()
	for _, ch := range "crdt is nice" {
		t1.InsertChar(ch)
	}
	t2, _ := t1.Fork()

	// Rewrite 'crdt is' with 'crdts are' in t2.
	t2.SetCursor(6) //             .-- place cursor here
	t2.DeleteChar() // c r d t _ i s
	t2.DeleteChar() //         ^ ^ ^
	t2.DeleteChar() //         and delete 3 chars
	for _, ch := range "s are" {
		t2.InsertChar(ch)
	}

	// Rewrite 'nice' with 'cool' in t1.
	for i := 0; i < 4; i++ {
		t1.DeleteChar()
	}
	for _, ch := range "cool" {
		t1.InsertChar(ch)
	}

	// Show contents of t1, t2, and then merge t2 into t1.
	fmt.Println("t1:", t1.ToString())
	fmt.Println("t2:", t2.ToString())
	t1.Merge(t2)
	fmt.Println("t1+t2:", t1.ToString())
	// Output:
	// t1: crdt is cool
	// t2: crdts are nice
	// t1+t2: crdts are cool
}

// Merging a set of overlapping changes may not produce intelligible results, but it's close
// enough to the intention of each party, and does not interrupt either to solve a merge conflict.
func ExampleCausalTree_overlap() {
	t1 := crdt.NewCausalTree()
	for _, ch := range "desserts" {
		t1.InsertChar(ch)
	}
	t2, _ := t1.Fork()

	// t1: desserts -> desert
	t1.DeleteCharAt(7)
	t1.DeleteCharAt(3)

	// t2: desserts -> dresser
	t2.DeleteCharAt(7)
	t2.DeleteCharAt(6)
	t2.InsertCharAt('r', 0)

	t1.Merge(t2)
	fmt.Println(t1.ToString())
	// Output: dreser
}

//
func ExampleCausalTree_ViewAt() {
	s0 := crdt.NewCausalTree() // S0 @ T1
	s0.InsertChar('a')         // S0 @ T2
	s1, _ := s0.Fork()         // S0 @ T3
	s1.InsertChar('b')         // S1 @ T4
	s1.InsertChar('c')         // S1 @ T5
	s2, _ := s0.Fork()         // S0 @ T4
	s2.InsertCharAt('x', -1)   // S2 @ T5
	s2.InsertChar('y')         // S2 @ T6
	s2.Merge(s1)               // S2 @ T7

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

	fmt.Println("Now: ", v1.ToString())
	fmt.Println("s2=0:", v2.ToString())
	fmt.Println("s1=0:", v3.ToString())
	fmt.Println("s0=0:", v4.ToString())
	// Output:
	// Now:  xyabc
	// s2=0: abc
	// s1=0: xya
	// s0=0: xy
}
