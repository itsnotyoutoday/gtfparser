/set v=hello
/def a = /echo [%{v}] [%v] [%{1}] [%{*}] [%{#}] [%1] [%2]
/a one two three
/a
/def b = /echo [%{-1}] [%{-2}] [%{L}] [%{L1}] [%{L2}] [%{-L1}] [%{-L}]
/b one two three
/def c = /echo [%{1-dflt}] [%{nosuch-fallback}] [%{v-no}]
/c
/c given
/def d = /echo 100%% $$x \\ \q
/d
/def e = /echo $[1+2] $[v] $[strlen(v)]
/e
/def g = /echo ${a}
/g
/def h = /echo %{0} %0
/h x
/def i = /let q=local%; /echo %q %{q}
/i
/def j = /return 42
/def k = /j%; /echo %?
/k
/def m = /echo a%;/echo b
/m
/def n = /echo "%{*}"
/n   spaced   out
/def o = /echo \101\x \0x41
/o
