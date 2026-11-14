; /return inside /eval ends the eval, not the macro around it; /break with no loop in the eval does not reach an outer loop
/def m1 = /eval /return 7%; /echo after-eval [%?]
/m1
/def m2 = /eval /break%; /echo after-break
/m2
/def m3 = /eval /return%; /echo r=[%?]
/m3
/def m4 = /eval -s0 /return 3%; /echo r=[%?]
/m4
; the input of a pipe lives only while its right side runs
/def src = /test echo("one")%; /test echo("two")
/def snk = /test tfread(a)%; /echo a=%a
/def two = /src %| /snk%; /test tfread(b)%; /echo b=[%b]
/two
