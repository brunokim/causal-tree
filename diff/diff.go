package diff

import (
	"fmt"
	"unicode/utf8"
)

type OpType int

const (
	Keep OpType = iota
	Insert
	Delete
)

type Operation struct {
	Op   OpType
	Char rune
	Dist int
}

// Example: abcd -> xabdy
//           s1      s2
//
// Legend:
//   ix = insert(x)
//   ka = keep(a)
//   dc = delete(c)
//
//          xabdy   xabdy   xabdy   xabdy   xabdy   xabdy
//  s1\s2   ^        ^        ^        ^        ^        ^
//        +-------+-------+-------+-------+-------+-------+
//        |       |       |       |       |       |       |
//  abcd  | ix 3  < ka 2  | da 3  | da 4  | iy 5  < da 4  |
//  ^     |       |      \|       |       |       |       |
//        +-------+-------+---^---+---^---+-------+---^---+
//        |       |       |       |       |       |       |
//  abcd  | ix 4  < ia 3  < kb 2  | db 3  | iy 4  < db 3  |
//   ^    |       |       |      \|       |       |       |
//        +-------+-------+-------+---^---+-------+---^---+
//        |       |       |       |       |       |       |
//  abcd  | ix 5  < ia 4  < ib 3  < dc 2  | iy 3  < dc 2  |
//    ^   |       |       |       |       |       |       |
//        +-------+-------+-------+---^---+-------+---^---+
//        |       |       |       |       |       |       |
//  abcd  | ix 4  < ia 3  < ib 2  < kd 1  | iy 2  < dd 1  |
//     ^  |       |       |       |      \|       |       |
//        +-------+-------+-------+-------+-------+---^---+
//        |       |       |       |       |       |       |
//  abcd  | ix 5  < ia 4  < ib 3  < id 2  < iy 1  < k0 0  |
//      ^ |       |       |       |       |       |       |
//        +-------+-------+-------+-------+-------+-------+

// Diff returns the sequence of insertions, deletions and insertions to transform s1 into s2.
func Diff(s1, s2 string) ([]Operation, error) {
	if !utf8.ValidString(s1) {
		return nil, fmt.Errorf("s1 is not a valid utf8 string")
	}
	if !utf8.ValidString(s2) {
		return nil, fmt.Errorf("s2 is not a valid utf8 string")
	}
	m, n := utf8.RuneCountInString(s1), utf8.RuneCountInString(s2)
	chars1 := make([]rune, m)
	for i, ch := range s1 {
		chars1[i] = ch
	}
	chars2 := make([]rune, n)
	for j, ch := range s2 {
		chars2[j] = ch
	}
	ops := make([]Operation, (m+1)*(n+1))
	coord := func(i, j int) int {
		return i*(n+1) + j
	}
	// Diff between s1 and an empty string: delete all chars
	for i, ch := range chars1 {
		ops[coord(i, n)] = Operation{
			Op:   Delete,
			Char: ch,
			Dist: m - i,
		}
	}
	// Diff between an empty string and s2: insert all chars
	for j, ch := range chars2 {
		ops[coord(m, j)] = Operation{
			Op:   Insert,
			Char: ch,
			Dist: n - j,
		}
	}
	// Compute all paths of operations that produce minimal edit distance.
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			ch1, ch2 := chars1[i], chars2[j]
			if ch1 == ch2 {
				// Chars are the same, keep it
				dist := ops[coord(i+1, j+1)].Dist
				ops[coord(i, j)] = Operation{
					Op:   Keep,
					Char: ch1,
					Dist: dist,
				}
			} else {
				// Pick smallest dist between possible sequences, preferring insert on a tie.
				op1 := ops[coord(i+1, j)]
				op2 := ops[coord(i, j+1)]
				if op2.Dist <= op1.Dist {
					// Insert char from s2.
					ops[coord(i, j)] = Operation{
						Op:   Insert,
						Char: chars2[j],
						Dist: 1 + op2.Dist,
					}
				} else {
					// Remove char from s1.
					ops[coord(i, j)] = Operation{
						Op:   Delete,
						Char: chars1[i],
						Dist: 1 + op1.Dist,
					}
				}
			}
		}
	}
	// Build sequence of operations.
	var operations []Operation
	var i, j int
	for i < m || j < n {
		op := ops[coord(i, j)]
		operations = append(operations, op)
		switch op.Op {
		case Keep:
			i++
			j++
		case Insert:
			j++
		case Delete:
			i++
		}
	}
	return operations, nil
}

// Distance returns the number of inserts/deletes to transform s1 into s2.
func Distance(s1, s2 string) (int, error) {
	operations, err := Diff(s1, s2)
	if err != nil {
		return 0, err
	}
	return operations[0].Dist, nil
}
