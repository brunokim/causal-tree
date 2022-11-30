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
		cursor := s1.Cursor()
		cursor.Insert('c')
		cursor.Insert('r')
		cursor.Insert('d')
		cursor.Insert('t')
		fmt.Println("set string:", t.Snapshot(), "- size:", s1.Len())
	}
	//
	{
		s2 := t.SetString()
		cursor := s2.Cursor()
		cursor.Insert('w')
		cursor.Insert('o')
		cursor.Insert('w')
		fmt.Println("set string:", t.Snapshot(), "- size:", s2.Len())
	}
	// Abstract walk over the current string, must know insertion order.
	{
		x1 := t.Value().(crdt.Container)
		cur := crdt.CursorAt(x1, 2)
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
		cursor := l1.Cursor()
		// counter element
		cursor.Insert().SetCounter().Increment(10)
		// nil element
		cursor.Insert()
		// string element
		strCursor := cursor.Insert().SetString().Cursor()
		strCursor.Insert('d')
		strCursor.Insert('o')
		strCursor.Insert('g')

		fmt.Println("set list:", t.Snapshot(), "- size:", l1.Len())
	}
	// Modify embedded counter
	{
		cursor := l1.Cursor()
		cursor.Index(0)
		cnt := cursor.Element().Value().(*crdt.Counter)
		cnt.Increment(40)
		cnt.Decrement(8)
		fmt.Println("modify counter:", t.Snapshot())
	}
	// Modify a string after it was pushed to the right by the previous insertion.
	{
		cur1 := l1.Cursor()
		cur1.Index(2)
		s2 := cur1.Element().Value().(*crdt.String)
		cur2 := s2.Cursor()
		cur2.Index(1)
		cur2.Delete()
		cur2.Delete()
		cur2.Insert('f')
		cur2.Insert('i')
		fmt.Println("modify string:", t.Snapshot(), "- size:", l1.Len())
	}
	// Delete elem
	{
		cursor := l1.Cursor()
		cursor.Index(1)
		cursor.Delete()
		fmt.Println("delete elem:", t.Snapshot(), "- size:", l1.Len())
	}
	// Delete counter and mutate after deletion.
	{
		cursor := l1.Cursor()
		cursor.Index(0)
		elem := cursor.Element()
		cnt := elem.Value().(*crdt.Counter)
		elem.Clear()
		cnt.Increment(27)
		fmt.Println("delete counter:", t.Snapshot(), "- size:", l1.Len())
	}
	// Insert char having deleted character as parent.
	{
		c0 := l1.Cursor()
		c0.Index(1)
		s1 := c0.Element().Value().(*crdt.String)
		c1 := s1.Cursor()
		c1.Index(2)
		c1.Insert('-')
		c1.Insert('z')
		c1.Insert('y')

		c2 := s1.Cursor()
		c2.Index(5)
		c2.Delete()

		c1.Insert('x')
		c2.Insert('w')
		fmt.Println("modify string:", t.Snapshot(), "- size:", l1.Len())
	}

	fmt.Println(t.PrintTable())
}
