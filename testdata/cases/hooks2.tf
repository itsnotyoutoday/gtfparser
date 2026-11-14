; the SEND hook sees plain text a macro body sends, and can swallow it
/def sendme = hello world
/def -hSEND -q -F sendq = /echo quiet send hook [%*]
/sendme
/def -hSEND sendh = /echo send hook [%*]
/sendme
/def -hSEND -F -p5 sendf = /echo fall [%*]
/def -h'SEND hello*' -p2 sendg = /echo glob [%*]
/sendme
/undef sendg
/undef sendf
/sendme
/echo -- hook argument text is the whole sent line
/def sendre2 = anything here
/def -mregexp -F -p9 -h'SEND ^(\w+) (\w+)$' sendre3 = /echo re P1=[%P1] P2=[%P2]
/sendre2
/echo -- hook off sends normally
/set hook=off
/sendme
/set hook=on
