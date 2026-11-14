/set zz=old
/set zz 
/eval /echo A: [%{zz}]
/set zz=old
/def -i tst = /set zz %*
/tst
/eval /echo B: [%{zz}]
/set zz=old
/set zz
/eval /echo C: [%{zz}]
