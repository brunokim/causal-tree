package crdt

// Counter represents a mutable integer value that may be incremented and decremented.
type Counter struct {
	tree   *CausalTree
	atomID atomID
	minLoc int
}

func (*Counter) isValue() {}

func (cnt *Counter) increment(x int32) {
	loc := cnt.tree.searchAtom(cnt.atomID, cnt.minLoc)
	cnt.minLoc = loc
	cnt.tree.addAtom(cnt.atomID, loc, incrementTag, x)
}

func (cnt *Counter) Increment(x int32) { cnt.increment(+x) }
func (cnt *Counter) Decrement(x int32) { cnt.increment(-x) }

func (cnt *Counter) Snapshot() int32 {
	loc := cnt.tree.searchAtom(cnt.atomID, cnt.minLoc)
	x, _, _ := cnt.tree.snapshotCounter(loc)
	return x
}
