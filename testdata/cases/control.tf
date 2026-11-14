/def w1 = /let i=0%; /while (i < 3) /echo i=%i%; /let i=$[i+1]%; /done
/test w1()
/def w2 = /let i=0%; /while (1) /let i=$[i+1]%; /if (i == 2) /echo skip%; /endif%; /if (i > 3) /break%; /endif%; /done%; /echo end %i
/test w2()
/def b2 = /let i=0%; /while (1) /let j=0%; /while (1) /let j=$[j+1]%; /if (j>2) /break 2%; /endif%; /done%; /done%; /echo ok
/test b2()
/def i1 = /if ({1}) /echo true%; /else /echo false%; /endif
/test i1(1)
/test i1(0)
/def i2 = /if ({1} == 1) /echo one%; /elseif ({1} == 2) /echo two%; /else /echo many%; /endif
/test i2(1)
/test i2(2)
/test i2(3)
/def r1 = /return 5
/def r2 = /echo before%; /return 7%; /echo after
/test E(r1())
/test E(r2())
/def f1 = /return {1} <= 1 ? 1 : {1} * f1({1} - 1)
/test E(f1(6))
/def fo = /for k 1 3 /echo k=%%k
/fo
/def n1 = /if (!{1}) /echo neg%; /endif
/n1 0
/def n2 = /if /test 1%; /then /echo listif%; /endif
/n2
/def n3 = /echo a %; /!echo b
/n3
