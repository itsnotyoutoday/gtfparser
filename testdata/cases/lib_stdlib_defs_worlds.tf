/def ism = /if /ismacro %1%; /then /echo yes:%1%; /else /echo no:%1%; /endif
/def foo1 = /echo foo1
/def foo2 = /echo foo2
/def bar1 = /echo bar1
/ism foo1
/ism foo2
/ism bar1
/purgedef foo*
/ism foo1
/ism foo2
/ism bar1
/purgedef
/ism bar1
/ism ism
/def -i keepme = /echo kept
/def -p5 -t"zzz*" trigx = /echo trig
/ism trigx
/untrig zzz*
/ism trigx
/def -t"abc*" trigy = /echo t
/undeft abc*
/ism trigy
/def -ag -t"gagme*" gagx
/ism gagx
/nogag gagme*
/ism gagx
/def -P1Cred -t"pp" -mregexp hil1
/def -F -ah -t"hh*" hil2
/nohilite hh*
/ism hil2
/addworld wtest1 localhost 4000
/addworld -Ttiny wtest2 mychar mypass example.org 1234
/addworld -T"lp" wtest3 example.com 23
/def r_lw = /echo [$(/@listworlds -s %*)]
/r_lw wtest1
/r_lw wtest*
/r_lw nosuchworld
/def r_lwc = /echo [$(/listworlds -c %*)]
/r_lwc wtest2
/purgeworld wtest1
/r_lw wtest*
/unworld wtest2
/r_lw wtest*
/def tstw = /test E(world_info("host", "wtest3"))
/test E(world_info("host", "wtest3"))
/test E(world_info("port", "wtest3"))
/test E(world_info("type", "wtest3"))
/test E(world_info("character", "wtest3"))
/addtiny wtiny1 c p h.example 5000
/test E(world_info("type", "wtiny1"))
/test E(world_info("character", "wtiny1"))
/test E(world_info("password", "wtiny1"))
/test E(world_info("host", "wtiny1"))
/test E(world_info("port", "wtiny1"))
/addtelnet wtel1 tel.example 23
/test E(world_info("type", "wtel1"))
/test E(world_info("host", "wtel1"))
/addlp wlp1 c p lp.example 5001
/test E(world_info("type", "wlp1"))
/adddiku wdk1 dk.example 4000
/test E(world_info("type", "wdk1"))
/addlpp wlpp1 lpp.example 4000
/test E(world_info("type", "wlpp1"))
/test E(is_open("wtiny1"))
/test E(fg_world())
