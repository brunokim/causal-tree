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

export function diff(s1, s2) {
  // Convert to codepoint array
  let chars1 = Array.from(s1),
    chars2 = Array.from(s2);
  let m = chars1.length,
    n = chars2.length;

  // Initialize (m+1) x (n+1) operation matrix
  let emptyArr = (len) => Array(len).fill();
  let ops = emptyArr(m + 1).map(() => emptyArr(n + 1));

  // Diff between s1 and an empty string: delete all chars from s1.
  chars1.forEach((ch, i) => {
    ops[i][n] = { op: "delete", ch: ch, dist: m - i };
  });
  // Diff between an empty string and s2: insert all chars from s2.
  chars2.forEach((ch, j) => {
    ops[m][j] = { op: "insert", ch: ch, dist: n - j };
  });
  // Diff between two empty strings: keep empty char.
  ops[m][n] = { op: "keep", ch: "", dist: 0 };

  // Compute all paths of operations that produce minimal edit distance.
  for (let i = m - 1; i >= 0; i--) {
    for (let j = n - 1; j >= 0; j--) {
      let ch1 = chars1[i],
        ch2 = chars2[j];
      let keepPath = ops[i + 1][j + 1];
      let insertPath = ops[i][j + 1];
      let deletePath = ops[i + 1][j];
      ops[i][j] = choosePath(ch1, ch2, keepPath, insertPath, deletePath);
    }
  }

  // Build sequence of operations.
  let i = 0,
    j = 0;
  let operations = [];
  while (i < m || j < n) {
    let op = ops[i][j];
    operations.push(op);
    if (op.op == "keep") {
      i++;
      j++;
    } else if (op.op == "delete") {
      i++;
    } else if (op.op == "insert") {
      j++;
    }
  }
  return operations;
}

function choosePath(ch1, ch2, keepPath, insertPath, deletePath) {
  if (ch1 == ch2) {
    return { op: "keep", ch: ch1, dist: keepPath.dist };
  }
  if (insertPath.dist <= deletePath.dist) {
    return { op: "insert", ch: ch2, dist: 1 + insertPath.dist };
  }
  return { op: "delete", ch: ch1, dist: 1 + deletePath.dist };
}
