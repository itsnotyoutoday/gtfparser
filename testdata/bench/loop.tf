/set max_instr=0
/def b1 = /let i=0%; /while (i < 300000) /let i=$[i+1]%; /done%; /echo done %i
/b1
