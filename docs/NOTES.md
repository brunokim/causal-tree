## Sequence

- @1: t1: Insert C
- @1: t2: Insert M
- @1: t3: Insert D
- @1: t4: Fork #2
- @1: t5: Delete "D" (t3@1)
- @1: t6: Delete "M" (t2@1)
- @1: t7: Insert T
- @1: t8: Insert R
- @1: t9: Insert L

- @2: t5: Fork #3
- @2: t6: Insert A
- @2: t7: Insert L
- @2: t8: Insert T

- @3: t6: Insert D
- @3: t7: Insert E
- @3: t8: Insert L

## Weave

Site #1:

      .---------------------------------. .---------------.
      v                                 | v               |
    [1|C 1]<-[1|T 7]<-[1|R 8]<-[1|L 9]  [1|M 2]<-[1|# 5]  [1|D 3]<-[1|# 6]

Site #2:

    [1|C 1]<-[1|M 2]<-[1|D 3]<-[2|A 6]<-[2|L 7]<-[2|T 8]

Site #3:

    [1|C 1]<-[1|M 2]<-[1|D 3]<-[3|D 6]<-[3|E 7]<-[3|L 8]

Site #1 + #2:

      .---------------------------------. .---------------. .---------------.
      v                                 | v               | v               |
    [1|C 1]<-[1|T 7]<-[1|R 8]<-[1|L 9]  [1|M 2]<-[1|# 5]  [1|D 3]<-[1|# 6]  [2|A 6]<-[2|L 7]<-[2|T 8]

Site #2 + #3:

                        .---------------------------------.
                        v                                 |
    [1|C 1]<-[1|M 2]<-[1|D 3]<-[2|A 6]<-[2|L 7]<-[2|T 8]  [3|D 6]<-[3|E 7]<-[3|L 8]

Site #1 + #2 + #3:

      .---------------------------------. .---------------. .---------------+--------------------------.
      v                                 | v               | v               |                          |
    [1|C 1]<-[1|T 7]<-[1|R 8]<-[1|L 9]  [1|M 2]<-[1|# 5]  [1|D 3]<-[1|# 6]  [2|A 6]<-[2|L 7]<-[2|T 8]  [3|D 6]<-[3|E 7]<-[3|L 8]

## Causal block

### Premises

1. an atom always appears to the left of its descendants
2. an atom always has a lower lamport timestamp than its descendants
3. causal blocks are always contiguous intervals

### Causal block algorithm proof

                          .----------------------------------------------------.
                          |         <--.                                       |
                          v            |                                       |
    [root ]  [atom1]  ... [parent] ... [head ]  [desc1]  [desc2]  ... [descN]  [other]
                                       --------------------------------------
                                                causal block of head

1. the first atom not in head's causal block will have a parent to the left of head
2. both head and this atom are part of this parent's causal block
3. therefore, head is necessarily a descendant of parent
4. therefore, head necessarily has a higher timestamp than parent
5. meanwhile, every atom in head's causal block will necessarily have a higher timestamp than head
6. thus: the first atom whose parent has a lower timestamp than head is past the end of the causal block

## Merging weaves

### Merging #2 into #1

    Iteration 0: i == j

    #1: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [1|# 6]
        ^i

    #2: [1|C 1]  [1|M 2]  [1|D 3]  [2|A 6]  [2|L 7]  [2|T 8]
        ^j

    Iteration 1-3: j predates i (both have same site)

    #1: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [1|# 6]
                 ^i       ^i       ^i

    #2: [1|C 1]  [1|M 2]  [1|D 3]  [2|A 6]  [2|L 7]  [2|T 8]
                 ^j

    Iteration 4: i == j

    #1: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [1|# 6]
                                            ^i

    #2: [1|C 1]  [1|M 2]  [1|D 3]  [2|A 6]  [2|L 7]  [2|T 8]
                 ^j

    Iteration 5: j predates i

    #1: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [1|# 6]
                                                     ^i

    #2: [1|C 1]  [1|M 2]  [1|D 3]  [2|A 6]  [2|L 7]  [2|T 8]
                          ^j

    Iteration 6: i == j

    #1: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [1|# 6]
                                                              ^i

    #2: [1|C 1]  [1|M 2]  [1|D 3]  [2|A 6]  [2|L 7]  [2|T 8]
                          ^j

    Iteration 7: concurrent change, sort their causal blocks

    #1: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [1|# 6]
                                                                       ^i-----

    #2: [1|C 1]  [1|M 2]  [1|D 3]  [2|A 6]  [2|L 7]  [2|T 8]
                                   ^j-----------------------

    Iteration 8: end

    #1: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [1|# 6]  [2|A 6]  [2|L 7]  [2|T 8]
                                                                                                         ^i

    #2: [1|C 1]  [1|M 2]  [1|D 3]  [2|A 6]  [2|L 7]  [2|T 8]
                                                            ^j

### Merging #3 into #2

    Iteration 0-2: i == j

    #2: [1|C 1]  [1|M 2]  [1|D 3]  [2|A 6]  [2|L 7]  [2|T 8]
        ^i       ^i       ^i

    #3: [1|C 1]  [1|M 2]  [1|D 3]  [3|D 6]  [3|E 7]  [3|L 8]
        ^j       ^j       ^j

    Iteration 3: concurrent change, sort their causal blocks

    #2: [1|C 1]  [1|M 2]  [1|D 3]  [2|A 6]  [2|L 7]  [2|T 8]
                                   ^i-----------------------

    #3: [1|C 1]  [1|M 2]  [1|D 3]  [3|D 6]  [3|E 7]  [3|L 8]
                                   ^j-----------------------

    Iteration 4: end

    #2: [1|C 1]  [1|M 2]  [1|D 3]  [2|A 6]  [2|L 7]  [2|T 8]  [3|D 6]  [3|E 7]  [3|L 8]
                                                                                       ^i

    #3: [1|C 1]  [1|M 2]  [1|D 3]  [3|D 6]  [3|E 7]  [3|L 8]
                                                            ^j

### Merging #1 into #2

    Iteration 0: i == j

    #2: [1|C 1]  [1|M 2]  [1|D 3]  [2|A 6]  [2|L 7]  [2|T 8]
        ^i

    #1: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [1|# 6]
        ^j

    Iteration 1: i predates j, insert remote causal block

    #2: [1|C 1]  [1|M 2]  [1|D 3]  [2|A 6]  [2|L 7]  [2|T 8]
                 ^i

    #1: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [1|# 6]
                 ^j-----------------------

    Iteration 2: i == j

    #2: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|D 3]  [2|A 6]  [2|L 7]  [2|T 8]
                                            ^i

    #1: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [1|# 6]
                                            ^j

    Iteration 3: i predates j, insert remote causal block

    #2: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|D 3]  [2|A 6]  [2|L 7]  [2|T 8]
                                                     ^i

    #1: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [1|# 6]
                                                     ^j-----

    Iteration 4: i == j

    #2: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [2|A 6]  [2|L 7]  [2|T 8]
                                                              ^i

    #1: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [1|# 6]
                                                              ^j

    Iteration 5: concurrent change, sort causal blocks

    #2: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [2|A 6]  [2|L 7]  [2|T 8]
                                                                       ^i-----------------------

    #1: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [1|# 6]
                                                                       ^j-----

    Iteration 6: end

    #2: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [1|# 6]  [2|A 6]  [2|L 7]  [2|T 8]
                                                                                                         ^i

    #1: [1|C 1]  [1|T 7]  [1|R 8]  [1|L 9]  [1|M 2]  [1|# 5]  [1|D 3]  [1|# 6]
                                                                              ^j

## Demo web interface

Textarea offers some properties to know its content and cursor state:

- `value`: actual textarea's content.
- `selectionStart`: position in text of the start of the selection range.
- `selectionEnd`: position in text of the end of the selection range.
- `selectionDirection`: direction of selection, either `none`, `forward` or `backward`.

If `selectionStart` = `selectionEnd`, then it's just an actual cursor.

When content changes, the following are the possible transitions. The character `|` means a cursor,
and `[]` means a selection.

1. Insertion: `abc|de -> abcx|de`
2. Deletion: `abc|de -> ab|de`
3. Cursor to forward selection: `abc|de -> abc[d]e`
4. Cursor to backward selection: `abc|de -> ab[c]de`
5. Grow forward selection: `abc[d]e -> abc[de]`
6. Grow backward selection: `ab[c]de -> a[bc]de`

Wow, it seems that there are _many_ more events: https://rawgit.com/w3c/input-events/v1/index.html#interface-InputEvent-Attributes

I was hoping I could just send the server the actual edit made by the user using the `input` event,
but it seems that I should keep using a diff algorithm server-side, because I can't reproduce all
these events.

## Operations to build other data structures

    {                     # map
      "key1": 1,          # int
      "key2": "str",      # str
      "key3": ['a', 'b'], # list
      "key4": {3, 2, 1},  # set
      "key5": 3.14,       # float
    }

    [map]
      |
      +-- [entry]
      |    |   '--[key]--[str]--['k']--['e']--['y']--['1']
      |    '------[val]--[int]--[+1]
      +-- [entry]
      |    |   '--[key]--[str]--['k']--['e']--['y']--['2']
      |    '------[val]--[str]--['s']--['t']--['r']
      +-- [entry]
      |    |   '--[key]--[str]--['k']--['e']--['y']--['3']
      |    '------[val]--[list]
      |                     '---[char]-['a']
      |                            '---[char]-['b']
      +-- [entry]
      |    |   '--[key]--[str]--['k']--['e']--['y']--['4']
      |    '------[val]--[set]
      |                    |
      |                    +----[int]--[+3]
      |                    +----[int]--[+2]
      |                    +----[int]--[+1]
      +-- [entry]
           |   '--[key]--[str]--['k']--['e']--['y']--['5']
           '------[val]--[float]-[+3.14]

- How to merge two maps with conflicting key mappings? Say,
  `{a: [1, 2]} + {a: [x, y]} => {a: [1, 2] + [x, y]}`, but what
  about `{a: 1} + {a: x}`?
  - Both can be kept, but only the first would be considered.
    The UI can note that there is a conflict that may need to
    be resolved or ignored.
    - This can be a strategy for every "illegal" state that
      can't be auto-merged like sequences.
- How to note if a set is add-wins or delete-wins? Same question
  for maps.
- How to note if an int conflict should be last-write-wins or
  addition? Or that a float conflict can take the mean?
- What about custom structs/named tuples? Or actual tuples?
- Can we express restrictions on a data structure, like accepted
  types on a list, or that a set has sorted order, or insertion
  order?

## API ideas

    type Container interface

    type Value interface

    type Map struct
    func NewMap() *Map
    func (*Map) AddEntry() *Entry
    func (*Map) ForEach(func(*Entry) bool)
    func (*Map) Entries() []*Entry
    func (*Map) Keys() []*Key
    func (*Map) Vals() []*Val

    type Entry struct
    func (*Entry) AddKey() *Key
    func (*Entry) AddVal() *Val
    func (*Entry) Key() *Key
    func (*Entry) Val() *Val

    type Key struct

    type Val struct

    type List struct
    func NewList() *List
    func (*List) AppendElem() *Elem
    func (*List) InsertElemAt(i int) *Elem
    func (*List) ForEach(func(*Elem) bool)
    func (*List) Elems() []*Elem

    type Elem struct
    func (*Elem) AddChild(child *Atom)
    func (*Elem) Child() *Atom

    type Str struct
    func NewStr() *Str
    func (*Str) AppendChar(ch rune)
    func (*Str) AppendChars(chars str)
    func (*Str) InsertCharAt(ch rune, i int)
    func (*Str) InsertCharsAt(chars str, i int)
    func (*Str) Str() str

    type Char struct { x rune }

    type Int32 struct { x int32 }
    type Int64 struct { x int64 }
    type Uint32 struct { x uint32 }
    type Uint64 struct { x uint64 }

    type Float32 struct { x float32 }
    type Float64 struct { x float64 }

    type Counter struct

    type Symbol struct

    type Record struct
    func NewRecord(Schema) *Record
    func (*Record) Set(name str, child *Atom)
    func (*Record) Get(name str) *Atom

Sample program:

    m := crdt.NewMap()

    e1 := m.AddEntry()
    e1.AddKey().Str().InsertChars("key1")
    e1.AddVal().Int(1)

    e2 := m.AddEntry()
    e2.AddKey().Str().InsertChars("key2")
    e2.AddVal().Str().InsertChars("str")

    e3 := m.AddEntry()
    e3.AddKey().Str().InsertChars("key3")
    l := e3.AddVal().List()
    l.AppendElem().Char('a')
    l.AppendElem().Char('b')

    var foo5
    m.ForEach(func (entry *Entry) bool {
        k, v := entry.Key(), entry.Val()
        key := k.(*Str).Str()
        if key != "key5" {
            return true
        }
        foo5 = v.(*Record).Get("foo")
        return false
    })

"Builder" program:

    // Up() returns the first container parent.
    // Entry, List, Map are containers
    m := crdt.NewMap()
        .AddEntry()
            .AddKey().Str().InsertChars("key1").Up()
            .AddVal().Int(1).Up()
            .Up()
        .AddEntry()
            .AddKey().Str().InsertChars("key2").Up()
            .AddVal().Str().InsertChars("str").Up()
            .Up()
        .AddEntry()
            .AddKey().Str().InsertChars("key3").Up()
            .AddVal().List()
                .AppendElem().Char('a').Up()
                .AppendElem().Char('b').Up()
                .Up()
            .Up()
        .Up()

Another "builder" program. Not sure how to implement it, or even if it's possible,
without breaking with invariants on a causal tree:

    t := crdt.NewTree()
    m1 := t.Map(
        t.Entry(
            t.Key(t.Str("key1")),
            t.Val(t.Int(1))),
        t.Entry(
            t.Key(t.Str("key2")),
            t.Val(t.Str("str"))),
        t.Entry(
            t.Key(t.Str("key3")),
            t.Val(t.List(
                t.Elem(t.Char('a')),
                t.Elem(t.Char('b'))))))

    elem  := t.Elem
    entry := t.Entry
    int_  := t.Int
    key   := t.Key
    list  := t.List
    map   := t.Map
    str   := t.Str
    val   := t.Val

    m2 := map(
        entry(
            key(str("key1")),
            val(int_(1))),
        entry(
            key(str("key2")),
            val(str("str"))),
        entry(
            key(str("key3")),
            val(list(
                elem(char('a')),
                elem(char('b'))))))

Learned that:

- All objects need to point to their corresponding atom, and from there find its children and insert locations.
- If we try to add only by holding the list's head, we're going to have O(n^2) behavior for multiple insertions.
  It's necessary to have some kind of cache with atom locations.
- We can't build objects from the bottom up easily, because the leafs must point to their parents, and they need to
  have a larger timestamp.
  The parent must be able to exist in an empty state before any children are appended to it.
