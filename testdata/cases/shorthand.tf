/def capl = /list %* %| /while (tfread(l) >= 0) /echo [%l]%; /done
/def capt = /capl -msimple -t'%*'
/echo -- /gag /hilite /trig family
/gag gagged one
/capt gagged one
/hilite hilited one
/capt hilited one
/set gpri=5
/gag gag pri five
/capt gag pri five
/set hpri=3
/hilite hilite pri three
/capt hilite pri three
/set gpri=0
/set hpri=0
/trig trig plain = /echo trig plain ran
/capt trig plain
/trigp 7 trigp pat = /echo trigp ran
/capt trigp pat
/trigc 50 trigc pat = /echo trigc ran
/capt trigc pat
/trigpc 4 25 trigpc pat = /echo trigpc ran
/capt trigpc pat
/trigpc x 25 bad
/trigpc 4 bad
/trigger trig plain
/trigger trigp pat
/echo -- patterns and bodies with spaces and equals
/trig  spaced   pattern  =  /echo spaced body
/capt spaced   pattern
/trigger spaced   pattern
/trig pat = with = equals
/capt pat
/trig *glob*star* = /echo glob star
/trigger xx glob yy star zz
/echo -- /trig with the matching variable
/set matching=regexp
/trig ^rx(\d+)$ = /echo rx %P1
/set matching=glob
/trigger rx42
/echo -- partial
/partial bolded
/capt bolded
/echo -- /hook forms
/hook CONNECT oth* = /echo c1
/hook CONNECT mud* = /echo c2 %*
/hook CONNECT|WORLD  w* = /echo c3
/hook NOSUCH = /echo x
/trigger -hCONNECT mud1
/trigger -hCONNECT other
/trigger -hWORLD wx
/echo -- /undeft /untrig /nogag /nohilite /unhook
/def -t'del1' -ag d1
/def -t'del1' -aB d2
/def -t'del2' d3 = /echo x
/undeft del1
/test E(ismacro("d1") + ismacro("d2") * 2 + ismacro("d3") * 4)
/untrig -ag del2
/test E(ismacro("d3"))
/undeft del2
/test E(ismacro("d3"))
/gag nogagme
/hilite nohiliteme
/nogag nogagme
/nohilite nohiliteme
/capt nogagme
/capt nohiliteme
/hook MORE = /echo more hook
/unhook MORE
/unhook MORE
/trigger -hMORE
/echo -- /purge
/def -t'pg1' pg1
/def -t'pg2' -p3 pg2
/def pg3 = x
/def -i pg4 = x
/purge pg*
/test E(ismacro("pg1") + ismacro("pg2") + ismacro("pg3") + ismacro("pg4"))
/purge -i pg4
/test E(ismacro("pg4"))
/def -t'pp1' pq1
/def -t'pp2' -p3 pq2
/def -t'pp3' -ag pq3
/purge -p3 pq*
/test E(ismacro("pq1") + ismacro("pq2") + ismacro("pq3"))
/purge -ag pq*
/test E(ismacro("pq1") + ismacro("pq2") + ismacro("pq3"))
/purge -msimple -t'pp1'
/test E(ismacro("pq1"))
/echo -- /undefn and numbers
/def un1 = x
/def un2 = x
/def un3 = x
/def numof = /list -s %1 %| /test tfread(l) %| /echo %l
/undef un2
/test E(ismacro("un1") + ismacro("un2") * 2 + ismacro("un3") * 4)
/echo -- /undef several and missing
/def u1 = x
/def u2 = x
/undef u1 nosuchmacro u2
/test E(ismacro("u1") + ismacro("u2"))
/echo -- reserved and illegal names
/def if = x
/def eval = x
/def 3d = x
/test E(ismacro("3d"))
/def a/b = x
/test E(ismacro("a/b"))
/def !x = x
/def @x = x
/def #x = x
/def -- name with spaces = x
/echo -- leading dash names
/def - -dash = x
/capl -dash
/echo -- keyword case
/def WHILE = x
/def ElseIf = x
