/def -i _b0 = /echo $[100000*100000]
/_b0
/def -i _b1 = /echo $[123456789012 + 1] $[-2147483648 + 0] $[+2147483648] $[1+4294967296]
/_b1
/def -i _b2 = /echo $[-1.5*2] $[1.5*2]
/_b2
/def -i _b3 = /echo $[5 < 7 =~ 1] $[1 < 2 !~ 0]
/_b3
/def -i _b4 = /echo $[pow (2,3)] $[strlen ("abc")]
/_b4
/def -i _b5 = /echo $[strlen("a","b")] x
/_b5
/def -i _b6 = /set y=5%;/echo $[y - --y]
/_b6
/def -i _b7 = /def b4 = /test shift()%;/echo %1 %{#}%;/b4 a b c
/_b7
/def -i _b8 = /echo $[1 ? 2:3] z
/_b8
/def -i _b9 = /shift -1%;/echo ok
/_b9
/def -i _b10 = /echo $[strncmp("abc","abd",-1)]
/_b10
