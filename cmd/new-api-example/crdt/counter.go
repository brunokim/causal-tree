package crdt

// Counter represents a mutable integer value that may be incremented and decremented.
type Counter struct {
	treePosition
}

func (*Counter) isValue() {}

func (cnt *Counter) increment(x int32) {
	pos := cnt.currPos()
	cnt.tree.addAtom(cnt.atomID, pos, incrementTag, x)
}

func (cnt *Counter) Increment(x int32) { cnt.increment(+x) }
func (cnt *Counter) Decrement(x int32) { cnt.increment(-x) }

func (cnt *Counter) Snapshot() int32 {
	pos := cnt.currPos()
	x, _, _ := cnt.tree.snapshotCounter(pos)
	return x
}
