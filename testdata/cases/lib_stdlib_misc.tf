/def r_rfact = /echo [$(/rfact %*)]
/def q_rfact = /let _r=$(/rfact %*)%; /echo {%{_r}}
/r_rfact 0
/r_rfact 1
/r_rfact 5
/r_rfact 10
/r_rfact -1
/r_rfact 12
/r_rfact 3.0
/r_rfact 20
/r_rfact 
/q_rfact 0
/def r_ifact = /echo [$(/ifact %*)]
/def q_ifact = /let _r=$(/ifact %*)%; /echo {%{_r}}
/r_ifact 0
/r_ifact 1
/r_ifact 5
/r_ifact 10
/r_ifact -1
/r_ifact 12
/r_ifact 20
/r_ifact 
/q_ifact 0
/def r_not = /echo [$(/not %*)]
/def q_not = /let _r=$(/not %*)%; /echo {%{_r}}
/r_not /test 1
/r_not /test 0
/r_not /echo hi
/r_not /test "a" =~ "b"
/r_not /true
/r_not /false
/q_not /test 1
/def r_rand = /echo [$(/rand %*)]
/def q_rand = /let _r=$(/rand %*)%; /echo {%{_r}}
/r_rand 1
/r_rand 1 1
/r_rand 5 5
/r_rand 0
/r_rand 1 2 3
/q_rand 1
/def tg = /toggle %*%; /echo [%{%1}]
/set X=1
/tg X
/tg X
/tg X
/set Y=
/tg Y
/tg Y
/set Z=hello
/tg Z
/tg Z
/tg W
/for i 1 3 /echo for:%i
/for i 3 1 /echo never
/for i 1 1 /echo once:%i
/for j 2 4 /echo [%j]
/for a 1 2 /for b 1 2 /echo %a%b
/for i 1 3 /echo %i%; /echo again%i
/for i -1 1 /echo neg:%i
/for i 1 3 /test echo(i * i)
/for i 1.5 3 /echo f:%i
/if /true%; /then /echo T%; /else /echo F%; /endif
/if /false%; /then /echo T%; /else /echo F%; /endif
/if /: arg arg%; /then /echo colon%; /endif
/test E(true())
/test E(false())
/def ism = /if /ismacro %1%; /then /echo yes:%1%; /else /echo no:%1%; /endif
/ism first
/ism nosuchmacro
/ism ism
/ism r_not
/ism echo
/def isv = /if /isvar %1%; /then /echo yes:%1%; /else /echo no:%1%; /endif
/set vv=3
/isv vv
/isv nosuchvar
/isv X
/test E(pi)
/test E(e)
/test E(pi * 2)
/test E(acos(0) * 2 == pi)
/test E(exp(1) == e)
/test E(strlen(pi))
/test E(ftime("%Y-%m-%d", 86400 * 365))
/test E(ftime("%H:%M:%S", 3661))
/test E(ftime("%a %b %e", 0))
/test E(ftime("", 0))
/test E(ftime("%%", 0))
/test E(ftime("%Y", 1e9))
/test E(ftime("%j", 86400 * 40))
/set time_format=%Y
/test E(strlen(ftime(time_format, time())))
/test E(systype())
/test E(columns() > 0)
/set tmp1=abc
/test E(tmp1 =~ "abc")
/def ee = /echo $[{1}]%; /echo $${1}
/ee 5
/def caser = /echo $[toupper({1})] $[tolower({2})]
/caser abc DEF
