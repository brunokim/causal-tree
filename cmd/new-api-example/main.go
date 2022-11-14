package main

import (
    "fmt"
    "strings"
)

type Container interface {
    Cursor() Cursor
}

type Cursor interface {
    Index(i int)
    Value() Value
}

type Value interface {
    Delete()
}

// ----

func ValueAt(c Cursor, i int) Value {
    c.Index(i)
    return c.Value()
}

// ----

type AtomTag int
const (
    deleteTag AtomTag = iota
    charTag
    elementTag
    incrementTag
    stringTag
    counterTag
    listTag
)

type AtomID int32

type Atom struct {
    id AtomID
    causeID AtomID

    tag AtomTag
    value int32 // rune for Char, increment for Increment
}

func (a Atom) printValue() string {
    switch a.tag {
    case deleteTag:
        return "\u232b"
    case charTag:
        return fmt.Sprintf("char %c", a.value)
    case incrementTag:
        return fmt.Sprintf("inc %d", a.value)
    case stringTag:
        return "string"
    case counterTag:
        return "counter"
    }
    return "(unknown)"
}

type CausalTree struct {
    atoms []Atom
}

func (t *CausalTree) Print() string {
    var lastID AtomID
    var sb strings.Builder
    for _, atom := range t.atoms {
        if atom.causeID != lastID {
            fmt.Fprintf(&sb, "%04d | %04d | %s\n", atom.causeID, atom.id, atom.printValue())
        } else {
            fmt.Fprintf(&sb, "     | %04d | %s\n", atom.id, atom.printValue())
        }
        lastID = atom.id
    }
    return sb.String()
}

// ----

func (t *CausalTree) searchAtom(id AtomID, minLoc int) int {
    if id == 0 {
        return -1
    }
    for i := minLoc; i < len(t.atoms); i++ {
        if t.atoms[i].id == id {
            return i
        }
    }
    return -1
}

func (t *CausalTree) addAtom(causeID AtomID, loc int, tag AtomTag, value int32) (AtomID, int) {
    if loc >= 0 && t.atoms[loc].id != causeID {
        panic(fmt.Errorf("cause loc-ID mismatch: loc=%d id=%d atoms[%d].id=%d", loc, causeID, loc, t.atoms[loc].id))
    }
    t.atoms = append(t.atoms, Atom{})
    copy(t.atoms[loc+2:], t.atoms[loc+1:])
    id := AtomID(len(t.atoms)+1)
    t.atoms[loc+1] = Atom{id, causeID, tag, value}

    return id, loc+1
}

func (t *CausalTree) youngerThan(i, j int) bool {
    return t.atoms[i].id > t.atoms[j].id
}

// Returns the size of a single char block.
func (t *CausalTree) charBlock(i int) (int, bool) {
    j := i + 1
    isDeleted := false
    loop:
    for j < len(t.atoms) && t.youngerThan(j, i) {
        atom := t.atoms[j]
        switch atom.tag {
        case deleteTag:
            isDeleted = true
            j++
        case charTag:
            break loop
        default:
            panic(fmt.Sprintf("char: unexpected tag: %v", atom.tag))
        }
    }
    return j-i, isDeleted
}

func (t *CausalTree) deleteAtom(atomID AtomID, minLoc int) int {
    loc := t.searchAtom(atomID, minLoc)
    t.addAtom(atomID, loc, deleteTag, 0)
    return loc
}

// ----

func (t *CausalTree) NewString() *String {
    id, loc := t.addAtom(0, -1, stringTag, 0)
    return &String{
        tree: t,
        atomID: id,
        minLoc: loc,
    }
}

func (t *CausalTree) NewCounter() *Counter {
    id, loc := t.addAtom(0, -1, counterTag, 0)
    return &Counter{
        tree: t,
        atomID: id,
        minLoc: loc,
    }
}

type TreeCursor struct {
    tree *CausalTree
    atomID AtomID
    minLoc int
}

func (t *CausalTree) Cursor() *TreeCursor {
    return &TreeCursor{t, 0, -1}
}

func (c *TreeCursor) Index(i int) {
    if i < 0 {
        panic("Invalid negative index")
    }
    t := c.tree
    loc := t.searchAtom(c.atomID, c.minLoc)

    cnt := -1
    for j := loc+1; j < len(t.atoms); j++ {
        atom := t.atoms[j]
        if atom.causeID == 0 {
            cnt++
            if cnt == i {
                loc = j
                break
            }
        }
    }
    if cnt < i {
        panic(fmt.Sprintf("tree: index out of range: %d (size=%d)", i, cnt))
    }
    c.minLoc = loc
    c.atomID = t.atoms[loc].id
}

func (c *TreeCursor) Value() Value {
    loc := c.tree.searchAtom(c.atomID, c.minLoc)
    return c.tree.valueOf(loc)
}

func (t *CausalTree) valueOf(i int) Value {
    atom := t.atoms[i]
    switch atom.tag {
    case stringTag:
        return &String{t, atom.id, i}
    case counterTag:
        return &Counter{t, atom.id, i}
    default:
        panic(fmt.Sprintf("valueOf: unexpected tag: %v", atom.tag))
    }
}

// ----

func (t *CausalTree) Snapshot() []interface{} {
    i := 0
    var result []interface{}
    for i < len(t.atoms) {
        obj, size, isDeleted := t.snapshot(i)
        if !isDeleted {
            result = append(result, obj)
        }
        i += size
    }
    return result
}

func (t *CausalTree) snapshot(i int) (interface{}, int, bool) {
    switch t.atoms[i].tag {
    case stringTag:
        return t.snapshotString(i)
    case counterTag:
        return t.snapshotCounter(i)
    default:
        panic(fmt.Sprintf("unexpected tag %d", t.atoms[i].tag))
    }
}

func (t *CausalTree) snapshotString(i int) (string, int, bool) {
    var sb strings.Builder
    j := i + 1
    isDeleted := false
    for j < len(t.atoms) && t.youngerThan(j, i) {
        atom := t.atoms[j]
        switch atom.tag {
        case deleteTag:
            isDeleted = true
            j++
        case charTag:
            ch, size, charDeleted := t.snapshotChar(j)
            if !charDeleted {
                sb.WriteRune(ch)
            }
            j += size
        default:
            panic(fmt.Sprintf("string: unexpected tag: %v", atom.tag))
        }
    }
    return sb.String(), j - i, isDeleted
}

func (t *CausalTree) snapshotChar(i int) (rune, int, bool) {
    size, isDeleted := t.charBlock(i)
    return rune(t.atoms[i].value), size, isDeleted
}

func (t *CausalTree) snapshotCounter(i int) (int32, int, bool) {
    var sum int32
    j := i + 1
    isDeleted := false
    for j < len(t.atoms) && t.youngerThan(j, i) {
        atom := t.atoms[j]
        switch atom.tag {
        case deleteTag:
            isDeleted = true
            j++
        case incrementTag:
            sum += atom.value
            j++
        default:
            panic(fmt.Sprintf("counter: unexpected tag: %v", atom.tag))
        }
    }
    return sum, j - i, isDeleted
}

// ----

type String struct {
    tree *CausalTree
    atomID AtomID
    minLoc int
}

type Char struct {
    tree *CausalTree
    atomID AtomID
    minLoc int
    value rune
}

func (s *String) StringCursor() *StringCursor {
    return &StringCursor{s.tree, s.atomID, s.minLoc}
}

func (s *String) Cursor() Cursor {
    return s.StringCursor()
}

func (s *String) Delete() {
    loc := s.tree.deleteAtom(s.atomID, s.minLoc)
    s.minLoc = loc
}

func (ch *Char) Delete() {
    loc := ch.tree.deleteAtom(ch.atomID, ch.minLoc)
    ch.minLoc = loc
}

// ----

type StringCursor struct {
    tree *CausalTree
    atomID AtomID
    minLoc int
}

func (c *StringCursor) Index(i int) {
    if i < 0 {
        panic("Invalid negative index")
    }
    t := c.tree
    loc := t.searchAtom(c.atomID, c.minLoc)
    c.minLoc = loc

    cnt := -1
    j := loc+1
    for j < len(t.atoms) && t.youngerThan(j, loc) {
        atom := t.atoms[j]
        switch atom.tag {
        case deleteTag:
            // String is already deleted, but do nothing
            j++
        case charTag:
            size, isDeleted := t.charBlock(j)
            if !isDeleted {
                cnt++
                if cnt == i {
                    loc = j
                    break
                }
            }
            j += size
        default:
            panic(fmt.Sprintf("string: unexpected tag: %v", atom.tag))
        }
    }
    if cnt < i {
        panic(fmt.Sprintf("string: index out of range: %d (size=%d)", i, cnt))
    }
    c.minLoc = loc
    c.atomID = t.atoms[loc].id
}

func (c *StringCursor) Char() *Char {
    loc := c.tree.searchAtom(c.atomID, c.minLoc)
    c.minLoc = loc
    return &Char{c.tree, c.atomID, loc, rune(c.tree.atoms[loc].value)}
}

func (c *StringCursor) Value() Value {
    return c.Char()
}

func (c *StringCursor) Insert(ch rune) (AtomID, int) {
    loc := c.tree.searchAtom(c.atomID, c.minLoc)
    id, charLoc := c.tree.addAtom(c.atomID, loc, charTag, int32(ch))
    c.atomID = id
    c.minLoc = charLoc
    return id, charLoc
}

func (c *StringCursor) Delete() {
    loc := c.tree.deleteAtom(c.atomID, c.minLoc)
    causeID := c.tree.atoms[loc].causeID
    isDeleted := false
    for i := loc-1; i >= 0; i-- {
        if causeID == 0 {
            // Cause is the root atom.
            loc = -1
            break
        }
        atom := c.tree.atoms[i]
        if atom.tag == deleteTag && atom.causeID == causeID {
            // Cause is also deleted.
            isDeleted = true
            continue
        }
        if atom.id != causeID {
            continue
        }
        if isDeleted {
            // Found cause, which is deleted. Reset cause to its parent, and keep searching.
            causeID = atom.causeID
            isDeleted = false
            continue
        }
        // Found existing cause, set its location.
        loc = i
        break
    }
    c.atomID = causeID
    c.minLoc = loc
}

// ----

type Counter struct {
    tree *CausalTree
    atomID AtomID
    minLoc int
}

func (cnt *Counter) increment(x int32) {
    loc := cnt.tree.searchAtom(cnt.atomID, cnt.minLoc)
    cnt.minLoc = loc
    cnt.tree.addAtom(cnt.atomID, loc, incrementTag, x)
}

func (cnt *Counter) Increment(x int32) { cnt.increment(+x) }
func (cnt *Counter) Decrement(x int32) { cnt.increment(-x) }

func (cnt *Counter) Delete() {
    loc := cnt.tree.deleteAtom(cnt.atomID, cnt.minLoc)
    cnt.minLoc = loc
}

// ----

func main() {
    t := new(CausalTree)
    // 
    s1 := t.NewString()
    {
        cursor := s1.StringCursor()
        cursor.Insert('c')
        cursor.Insert('r')
        cursor.Insert('d')
        cursor.Insert('t')
        fmt.Println(t.Snapshot())
    }
    // 
    s2 := t.NewString()
    {
        cursor := s2.StringCursor()
        cursor.Insert('w')
        cursor.Insert('o')
        cursor.Insert('w')
        fmt.Println(t.Snapshot())
    }
    // Abstract walk, must know insertion order.
    {
        x1 := ValueAt(t.Cursor(), 1).(Container)
        x2 := ValueAt(x1.Cursor(), 2)
        x2.Delete()
        fmt.Println(t.Snapshot())
    }
    // Insert a counter
    cnt := t.NewCounter()
    {
        cnt.Increment(45)
        cnt.Decrement(3)
        fmt.Println(t.Snapshot())
    }
    // Modify a string after it was pushed to the right by the previous insertion.
    {
        cursor := s2.StringCursor()
        cursor.Index(1)
        cursor.Delete()
        cursor.Delete()
        cursor.Insert('a')
        cursor.Insert('w')
        fmt.Println(t.Snapshot())
    }
    fmt.Println(t.Print())
}
