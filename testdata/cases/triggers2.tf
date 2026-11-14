; priority, fall-thru, shots, probability, conditions
/def -t'*foo*' -p5 -F f5 = /echo f5
/def -t'*foo*' -p4 g4 = /echo g4
/def -t'*foo*' -p4 -F f4 = /echo f4
/def -t'*foo*' -p3 -F f3 = /echo f3 never
/echo -- fall-thru ordering
/trigger a foo b
/echo -- result count
/def cnt = /test E(1+trigger_count)
/def -t'count me' -q quiet1 = /echo quiet ran
/def -t'count me' -p0 -F loud1 = /echo loud ran
/def tc = /trigger count me
/tc
/def rt = /test E({1}) %; /trigger count me
/echo -- priority 0 loses to 1; a negative priority becomes the default 1
/def -t'pz' -p0 pz0 = /echo pz0
/def -t'pz' -p-2 pzm2 = /echo pzm2
/trigger pz
/undef pzm2
/trigger pz
/echo -- one shots and n shots
/def -t'once' -1 o1 = /echo o1 ran
/def -t'thrice' -n3 o3 = /echo o3 ran
/trigger once
/trigger once
/trigger thrice
/trigger thrice
/trigger thrice
/trigger thrice
/test E(ismacro("o1"))
/test E(ismacro("o3"))
/echo -- probability
/def -t'never' -c0 pr0 = /echo never!
/def -t'always' -c100 pr100 = /echo always
/trigger never
/trigger always
/echo -- conditions
/set enabled=1
/def -t'cond *' -E'enabled' ce = /echo cond %-1
/def -t'cond *' -p0 -F ce0 = /echo cond fallback %-1
/trigger cond one
/set enabled=0
/trigger cond two
/echo -- -E does not stop lower priority
/def -t'lp' -p2 -E'0' hi = /echo hi
/def -t'lp' -p1 lo = /echo lo
/trigger lp
/echo -- self undef and redefine in a trigger
/def -t'selfdel' -F -p5 sd = /undef sd%; /echo sd ran
/trigger selfdel
/trigger selfdel
/def -t'chain' -F -p5 ch1 = /def -t'chain' -p1 ch2 = /echo ch2 defined during search%; /echo ch1
/trigger chain
/trigger chain
/echo -- trigger arguments
/def -t'args *' ta = /echo [%0] [%1] [%2] [%*] [%{#}]
/trigger args one two three
/def -mregexp -t'^(\w+)@(\w+)\.(\w+)$' re1 = /echo [%0] P0=%P0 P1=%P1 P2=%P2 P3=%P3 PL=[%PL] PR=[%PR]
/trigger bob@example.com
/def -mregexp -t'^ab(x)?(c)d' re2 = /echo P1=[%P1] P2=[%P2] PL=[%PL] PR=[%PR]
/trigger abcd
/echo -- nested trigger sees its own and outer regs
/def -mregexp -t'outer (\w+)' ro = /echo outer %P1%; /trigger inner xyz
/def -mregexp -t'inner (\w+)' ri = /echo inner %P1
/trigger outer abc
/echo -- match styles
/def -msimple -t'SimPle *' st1 = /echo simple matched
/def -msubstr -t'sub' st2 = /echo substr matched
/def -mregexp -t'^reg' st3 = /echo regexp matched
/trigger SimPle *
/trigger SimPle x
/trigger a sub string
/trigger Sub
/trigger regular
/trigger not reg
/echo -- unnamed macro triggers
/def -t'unnamed' = /echo unnamed ran %0
/trigger unnamed
/echo -- empty body
/def -t'nobody' nb
/def -t'nobody' -q -F -p3 nb2 = /echo nb2
