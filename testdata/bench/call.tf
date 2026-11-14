/set max_instr=0
/def sq = /return {1} * {1}
/def b3 = /let i=0%; /let t=0%; /while (i < 100000) /test t := t + sq(i)%; /test i := i + 1%; /done%; /echo %t
/b3
