# a comment line
/echo after-hash
/def -i _b1 = /echo [$(/echo x%;/echo y)]
/_b1
/def -i _b2 = /def e1 = /echo [%{-0}]%;/e1 a
/_b2
/def -i _b3 = /def e1 = /echo [%{#-x}]%;/e1 a
/_b3
/def -i _b4 = /def e2 = /echo [%-]%;/e2 a
/_b4
/def -i _b5 = /def e2 = /echo [%{1:}]%;/e2 a
/_b5
/def -i _b6 = /def e2 = /echo [%{ 1}]%;/e2 a
/_b6
/def -i _b7 = /echo $[strip_attr("abc")] [$[decode_attr("a",1)]] [$[encode_attr("abc")]] $[decode_attr("@{u}x@{n}y")]
/_b7
/def -i _b8 = /echo $[strcmp("a","b")] $[substr("hello",-2)] $[substr("hello",2,-1)]
/_b8
/def -i _b9 = /for i 1.5 3 /echo %i
/_b9
/def -i _b10 = /if 0%; /then /echo yes%; /else /echo no%; /endif
/_b10
/def -i _b11 = /def la = /let a=1%;/let b=2%;/echo [$[la()]]
/_b11
