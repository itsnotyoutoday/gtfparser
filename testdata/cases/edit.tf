/def snk = /while (tfread(l) >= 0) /echo [%l]%; /done
/def hsnk = /while (tfread(l) >= 0) /let i=$[strstr(l, " -h ")]%; /echo [$[i >= 0 ? strcat(substr(l, 0, i), substr(l, i + 3)) : l]]%; /done
/def cap = /%* %| /hsnk
/def e1 = /echo original
/def -t'abc*' -p3 e2 = /echo trig %*
/def -h'CONNECT w*' e3 = /echo hook
/def -t'x(y)z' -mregexp e4 = /echo re
/def -n2 -t'q*' e5 = /echo shots
/def e6 = /echo six
/e1
/def ed = \
  /edit e1 = /echo changed%; /echo r=%?%; \
  /e1%; \
  /edit -p9 e2%; /echo r=%?%; \
  /cap list e2%; \
  /edit -t'def*' e2 = /echo newtrig%; /echo r=%?%; \
  /cap list e2%; \
  /edit -hDISCONNECT e3%; /echo r=%?%; \
  /cap list e3%; \
  /edit -mglob e4%; /echo r=%?%; \
  /cap list e4%; \
  /edit -mregexp -t'a(b)' e4%; /echo r=%?%; \
  /cap list e4%; \
  /edit -n5 -F e5%; /echo r=%?%; \
  /cap list e5%; \
  /edit -f e5%; /cap list e5%; \
  /edit -q e2%; /echo r=%?%; /cap list e2%; \
  /edit -w'nosuchworld' e2%; /echo r=%?%; \
  /edit nosuchmacro = x%; /echo r=%?%; \
  /edit%; /echo r=%?%; \
  /edit -t'x'%; /echo r=%?%; \
  /edit -z e2%; /echo r=%?%; /edit e2 -p9%; /echo r=%?
/ed
/echo --undefn
/def u1 = /echo u1
/def u2 = /echo u2
/def u3 = /echo u3
/def un = \
  /cap list -s u*%; \
  /undefn $[ismacro("u2")]%; /echo r=%?%; \
  /cap list -s u*%; \
  /undefn 999999%; /echo r=%?%; \
  /undefn x%; /echo r=%?%; \
  /undefn%; /echo r=%?%; \
  /undefn -1%; /echo r=%?
/un
/def un2 = /let a=$[ismacro("u1")]%; /let b=$[ismacro("u3")]%; /undefn %a %b 999998%; /echo r=%?%; /cap list -s u*
/un2
