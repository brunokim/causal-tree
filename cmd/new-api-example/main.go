package main

import (
	"fmt"

	"github.com/brunokim/causal-tree/new-api-sample/crdt"
)

func main() {
	// Check expected types
	{
		var t *crdt.CausalTree
		_ = crdt.Register(t)
		_ = crdt.Value(t)

		var s *crdt.String
		_ = crdt.Container(s)
		_ = crdt.Value(s)

		var scur *crdt.StringCursor
		_ = crdt.Cursor(scur)

		var cnt *crdt.Counter
		_ = crdt.Value(cnt)

		var l *crdt.List
		_ = crdt.Container(l)
		_ = crdt.Value(l)

		var lcur *crdt.ListCursor
		_ = crdt.Cursor(lcur)

		var elem *crdt.Elem
		_ = crdt.Register(elem)
	}
	t := new(crdt.CausalTree)
	//
	{
		s1 := t.SetString()
		cursor := s1.StringCursor()
		cursor.Insert('c')
		cursor.Insert('r')
		cursor.Insert('d')
		cursor.Insert('t')
		fmt.Println("set string:", t.Snapshot())
	}
	//
	{
		s2 := t.SetString()
		cursor := s2.StringCursor()
		cursor.Insert('w')
		cursor.Insert('o')
		cursor.Insert('w')
		fmt.Println("set string:", t.Snapshot())
	}
	// Abstract walk, must know insertion order.
	{
		x1 := t.Value().(crdt.Container)
		cur := x1.Cursor()
		cur.Index(2)
		cur.Delete()
		fmt.Println("delete $.2:", t.Snapshot())
	}
	// Insert a counter
	{
		cnt := t.SetCounter()
		cnt.Increment(45)
		cnt.Decrement(3)
		fmt.Println("set counter:", t.Snapshot())
	}
	// Insert list
	l1 := t.SetList()
	{
		cursor := l1.ListCursor()
		// counter element
		cursor.Insert().SetCounter().Increment(10)
		// nil element
		cursor.Insert()
		// string element
		strCursor := cursor.Insert().SetString().StringCursor()
		strCursor.Insert('d')
		strCursor.Insert('o')
		strCursor.Insert('g')

		fmt.Println("set list:", t.Snapshot())
	}
	// Modify embedded counter
	{
		cnt := crdt.ElementAt(l1.Cursor(), 0).(*crdt.Elem).Value().(*crdt.Counter)
		cnt.Increment(40)
		cnt.Decrement(8)
		fmt.Println("modify counter:", t.Snapshot())
	}
	// Modify a string after it was pushed to the right by the previous insertion.
	{
		s2 := crdt.ElementAt(l1.Cursor(), 2).(*crdt.Elem).Value().(*crdt.String)
		cursor := s2.StringCursor()
		cursor.Index(1)
		cursor.Delete()
		cursor.Delete()
		cursor.Insert('f')
		cursor.Insert('i')
		fmt.Println("modify string:", t.Snapshot())
	}
	// Delete elem
	{
		cursor := l1.ListCursor()
		cursor.Index(1)
		cursor.Delete()
		fmt.Println("delete elem:", t.Snapshot())
	}
	// Delete counter and mutate after deletion.
	{
		cursor := l1.ListCursor()
		cursor.Index(0)
		elem := cursor.Element()
		cnt := elem.Value().(*crdt.Counter)
		elem.Clear()
		cnt.Increment(27)
		fmt.Println("delete counter:", t.Snapshot())
	}
	// Insert char having deleted character as parent.
	{
		s1 := crdt.ElementAt(l1.Cursor(), 1).(*crdt.Elem).Value().(*crdt.String)
		c1 := s1.StringCursor()
		c1.Index(2)
		c1.Insert('-')
		c1.Insert('z')
		c1.Insert('y')

		c2 := s1.StringCursor()
		c2.Index(5)
		c2.Delete()

		c1.Insert('x')
		c2.Insert('w')
		fmt.Println("modify string:", t.Snapshot())
	}

	fmt.Println(t.PrintTable())
}
