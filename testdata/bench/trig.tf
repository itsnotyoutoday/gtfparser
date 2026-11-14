/set max_instr=0
/def -t'foo*' t1 = /test x := 1
/def -mregexp -t'^bar (\d+)' t2 = /test y := {P1}
/def b5 = /let i=0%; /while (i < 20000) /trigger foo bar%; /trigger bar 12%; /test i := i + 1%; /done%; /echo ok
/b5
