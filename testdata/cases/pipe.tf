/def src = /test echo("one")%; /test echo("two")%; /test echo("three")
/def snk1 = /while (tfread(l) >= 0) /echo got:%l%; /done
/def snk2 = /test tfread(a)%; /test tfread(b)%; /echo a=%a b=%b
/def snk3 = /echo rd=$[tfread(z)] z=%z
/echo --1
/src %| /snk1
/echo --2
/src %| /snk2
/echo --3
/src %| /snk3
/echo --4
/def two = /src %| /snk1
/two
/echo --5
/def three = /src %| /snk1%; /echo after
/three
