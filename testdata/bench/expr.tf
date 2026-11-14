/set max_instr=0
/set x=0
/def b2 = /let i=0%; /while (i < 200000) /test x := x + i * 2 - (i / 3)%; /test i := i + 1%; /done%; /echo %x
/b2
