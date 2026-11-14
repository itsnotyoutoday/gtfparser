; library commands the engine implements itself: word and string helpers
/def -i T = /eval %*
/T /echo first=[$(/first a b c)] [$(/first)] [$(/first  x)]
/T /echo rest=[$(/rest a b c)] [$(/rest a)] [$(/rest)] [$(/rest a   b   c)]
/T /echo last=[$(/last a b c)] [$(/last z)] [$(/last)]
/T /echo nth=[$(/nth 2 a b c)] [$(/nth 1 a b c)] [$(/nth 3 a b c)] [$(/nth 0 a b)] [$(/nth -1 a b)]
/T /echo escape=[$(/escape ' it's a "test" \ ok)] [$(/escape)] [$(/escape abc)] [$(/escape a abcabc)] [$(/escape *? a*b?c)]
/T /echo replace=[$(/replace a b banana)] [$(/replace an  XX  banana split)] [$(/replace x y)]
/def -i sp = /split %*%; /echo split=[%P1][%P2]
/sp k = v w
/sp k=v
/sp  key   value here
/sp =v
/sp nothing
/T /echo func=[$[first("x y", "z")]] [$[rest("p", "q", "r")]] [$[last("1", "2")]] [$[nth(2, "a", "b")]]
/set tg=0
/toggle tg
/T /echo tg=%tg
/toggle tg
/T /echo tg=%tg
/T /echo expr=[$(/expr 3 * 4)] [$(/expr 7 - 2)] [$(/expr)]
/not /test 0
/T /echo not0=%?
/not /test 1
/T /echo not1=%?
/not /return 5
/T /echo not5=%?
/for i 1 3 /echo for-%i
/for i 3 1 /echo never-%i
/for j -1 1 /echo [%j]
/def -i fa = /for i 1 2 /for j 1 2 /echo %i%j
/fa
/def -i fb = /for k 1 5 /if (k == 3) /break%; /endif%; /echo k=%k
/fb
/T /echo isvar=[$(/isvar tg)]
/isvar nosuchvarxx
/T /echo isvar-none=%?
/isvar tg
/T /echo isvar-yes=%?
/ismacro for
/T /echo ismacro-yes=%?
/ismacro nosuchmacroxx
/T /echo ismacro-no=%?
/T /echo ismacro-fn=[$[ismacro("first")]] [$[ismacro("nosuchmacroxx")]] [$[ismacro("-i first")]] [$[ismacro("-t'x'")]]
/T /echo vercmp=[$[vercmp("5.0 beta 8", "5.0 beta 8")]] [$[vercmp("5.0 beta 8", "5.0 beta 9")]] [$[vercmp("5.1 alpha 1", "5.0 beta 9")]] [$[vercmp("5.0 stable 1", "5.0 beta 9")]]
/T /echo vercmp-bad=[$[vercmp("x", "5.0 beta 8")]] [$[vercmp("5.0 beta 8", "5.0")]]
/T /echo rand=[$[rand(1)]] [$[rand(5, 5)]]
/def -i rr = /rand %*
/rr 7 7
/rr 3 3
/runtime /echo inside
/T /echo pi=%pi e=%e
/T /echo time=[$(/time %Y)]
