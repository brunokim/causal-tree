
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
