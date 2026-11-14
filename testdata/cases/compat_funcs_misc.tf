/def -i _b0 = /def a2 = /echo two%;/set zzz=1%;/echo [$[ismacro('a2') > 0]] [$[isvar('zzz')]] [$[ismacro('nosuch')]] [$[isvar('nosuchv')]]
/_b0
/def -i _b1 = /def -t'ant*' t1 = /echo t1%;/untrig ant*%;/trigger ant%;/echo done
/_b1
/def -i _b2 = /def -t'zap' zz = /echo zap%;/def -t'zap2' zz2 = /echo zap2%;/nogag zap%;/purgedef zz*%;/trigger zap%;/trigger zap2%;/echo done
/_b2
/def -i _b3 = /echo $[set("qq=1")] $[qq] $[def("-t'zap' zq = /echo zap")>0]%;/trigger zap
/_b3
/def -i _b4 = /def dd1 = x%;/def dd2 = /echo [%?]%;/dd2%;/echo $[dd1 > 0]
/_b4
/def -i _b5 = /def -1 -t'once' o1 = /echo only once%;/trigger once%;/trigger once
/_b5
/def -i _b6 = /def a=x%;/def b=y%;/test undef("a b nosuch")%;/echo ok
/_b6
/def -i _b7 = /gag *spam*%;/list -s -mglob *spam*%;/hilite *eggs*%;/list *eggs*
/_b7
/def -i _b8 = /echo $[limit()] $[version()>0] $[features("256colors")]
/_b8
/def -i _b9 = /test E(shift())%;/echo $[mod(5,0)]x
/_b9
/def -i _b10 = /echo $[ftime("%s",1710504000)] $[ftime("%Y-%m-%d %H:%M:%S",86400*365)] $[ftime("%a %b %e %j %y %p %I",1710504000)]
/_b10
