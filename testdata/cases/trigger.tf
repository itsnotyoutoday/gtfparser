/def -t'hello *' t1 = /echo t1: %1 | %2 | %* | %{#}
/trigger hello world there
/trigger nothing here
/def -mregexp -t'^(\w+) says (.*)$' t2 = /echo t2: %{P1} / %{P2} / %{PL} / %{PR} / %1
/trigger Bob says hi there
/def -msimple -t'exact line' t3 = /echo t3
/trigger exact line
/trigger exact line more
/def -mglob -t'*foo*' -p5 t4 = /echo t4 high
/def -mglob -t'*foo*' -p1 t5 = /echo t5 low
/trigger a foo b
/def -mglob -t'*bar*' -p5 -F t6 = /echo t6 fall
/def -mglob -t'*bar*' -p5 t7 = /echo t7 second
/trigger bar
/def -mglob -t'one shot' -n1 t8 = /echo t8 once
/trigger one shot
/trigger one shot
/set flag=1
/def -mglob -t'cond' -E'flag' t9 = /echo t9 cond
/trigger cond
/set flag=0
/trigger cond
/def -mglob -t'[abc]x{1|2}' t10 = /echo t10
/trigger bx1
/trigger cx2
/trigger dx1
/def -mregexp -t'(a+)(b*)' t11 = /echo t11 [%{P0}] [%{P1}] [%{P2}]
/trigger xaabbby
/def -mregexp -t'HELLO' t12 = /echo t12 ci
/trigger hello
/def -mregexp -t'Hello' t13 = /echo t13 cs
/trigger hello
/trigger Hello
/def -t'GLOB*' t14 = /echo t14
/trigger globby
/trigger GLOBby
