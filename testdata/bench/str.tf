/set max_instr=0
/def b4 = /let i=0%; /let s=%; /while (i < 100000) /test s := strcat(substr("hello world", mod(i, 5), 3), toupper("ab"))%; /test i := i + 1%; /done%; /echo %s
/b4
