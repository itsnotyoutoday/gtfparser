/def sp = /split %*%; /echo [%P1][%P2]
/sp name = value
/def sp2 = /split %*
/def sp3 = /sp2 %*%; /echo [%P1][%P2]
/sp3 k = v w
/def -i _b2 = /echo $[regmatch("(a)(b)?","a")] $[regmatch("(a)(b)?(c)","ac")] $[regmatch("(","x")] $[regmatch("x(y)?z","xz")]
/_b2
/def -i _b3 = /def a3 = /echo trail %{nosuch}%;/a3
/_b3
/def b4 = /test shift()%;/echo %1 %{#}
/b4 a b c
/def -i _b5 = /def nn = /echo [$(/nth 2 alpha beta gamma)]%;/nn
/_b5
/def -i _b6 = /echo $[ismacro("nn")] $[isvar("zzz")]
/_b6
/def -i _b7 = /def r0 = /return%;/echo [$[r0()]] [$[whatis(r0())]]
/_b7
/def -i _b8 = /def h = /echo [%?]%;/h
/_b8
/def -i _b9 = /def d1 = x%;/def d2 = /let a=1%;/echo [$[d2()]] [$[d1()]]
/_b9
