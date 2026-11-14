/def m1 = /set q=1
/def m2 = /let q=1
/def m3 = /echo hi
/def m5 = /set q=1%;/echo x
/def m6 = /if (1) /set q=2%; /endif
/def m7 = /unset q
/def m8 = /undef zzzz
/def m9 = /m1
/def m10 = /set q=1%;x
/def tst = /echo [$[m1()]] [$[m2()]] [$[m3()]] [$[m5()]] [$[m6()]] [$[m7()]] [$[m8()]] [$[m9()]] [$[m10()]]%;/m1%;/echo [%?]
/tst

/def m11 = /test a := 7%;/let i=2
/def m12 = /m11
/def m13 = /m1
/def tst2 = /echo [$[m11()]] [$[m12()]] [$[m13()]]
/tst2
