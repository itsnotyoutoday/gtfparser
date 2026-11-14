/test E(strlen(pi))
/def selfdef = /echo ${self}
/def rtest = /return 5
/def rtest2 = /result 6
/def rtest3 = /return
/def rtest4 = /result
/test E(rtest())
/test E(rtest2())
/test E(rtest3())
/test E(rtest4())
/test E(rtest() + rtest2())
/def ckres = /rtest%; /echo [%?]%; /rtest2%; /echo [%?]%; /rtest3%; /echo [%?]%; /rtest4%; /echo [%?]
/ckres
