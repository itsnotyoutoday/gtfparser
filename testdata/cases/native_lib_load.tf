; /require, /loaded, /cat, /paste and friends
/def -i T = /eval %*
/test _f := tfopen("/tmp/gtfnl_a.tf", "w")
/test tfwrite(_f, "/loaded gtfnl_a.tf")
/test tfwrite(_f, "/echo loading-a")
/test tfclose(_f)
/require /tmp/gtfnl_a.tf
/require /tmp/gtfnl_a.tf
/T /echo libs=[%_loaded_libs]
/test _f := tfopen("/tmp/gtfnl_b.tf", "w")
/test tfwrite(_f, "/loaded gtfnl_b.tf")
/test tfwrite(_f, "/echo loading-b")
/test tfclose(_f)
/load /tmp/gtfnl_b.tf
/load /tmp/gtfnl_b.tf
/T /echo libs=[%_loaded_libs]
/def -i srcl = /test echo("/echo cat-one")%; /test echo("/echo cat-two")
/def -i pp1 = /srcl %| /cat
/echo --1
/pp1
/def -i srcp = /test echo("/echo pasted-one")%; /test echo("   ")%; /test echo("/echo pasted-two")%; /test echo("/echo pasted-three")
/def -i pp2 = /srcp %| /paste -x
/echo --2
/pp2
/def -i pp3 = /srcp %| /paste -q -x
/echo --3
/pp3
/def -i pp4 = /srcp %| /paste -qpx
/echo --4
/pp4
/def -i pp5 = /srcp %| /paste -qtx
/echo --5
/pp5
/def -i pp6 = /srcp %| /paste -qsx
/echo --6
/pp6
/def -i srcq = /test echo("/echo before")%; /test echo(".")%; /test echo("/echo after")
/def -i pp7 = /srcq %| /paste -qx
/echo --7
/pp7
/def -i srce = /test echo("/echo e1")%; /test echo("/endx")%; /test echo("/echo e2")
/def -i pp8 = /srce %| /paste -qx -e/endx
/echo --8
/pp8
/def -i srca = /test echo("/echo a1")%; /test echo("/abortx")%; /test echo("/echo a2")
/def -i pp9 = /srca %| /paste -qx -a/abortx
/echo --9
/pp9
/T /echo done
/paste -px -qn
/paste -qx -w
