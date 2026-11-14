/set wrapsize=80
/def snk = /while (tfread(l) >= 0) /echo [%l]%; /done
/def cap = /%* %| /snk
/def mk = \
  /test addworld("foo", "tiny", "host.example", "4000", "bob", "sekret", "", "", "")%; \
  /test addworld("bar", "")%; \
  /test addworld("baz", "lp", "h2", "23", "", "", "mf", "xe", "src")%; \
  /test addworld("default", "", "", "", "me", "pw", "")%; \
  /test addworld("Qux", "diku", "q.example", "5555", "a'b", "c d", "", "p", "")
/mk
/echo --list
/cap listworlds
/cap listworlds -s
/cap listworlds -c
/cap listworlds -T tiny
/cap listworlds -Tl*
/cap listworlds -s -Sh
/cap listworlds -s -St
/cap listworlds -s -Sp
/cap listworlds -s -Sc
/cap listworlds -s -S-
/cap listworlds -s b*
/cap listworlds -s -mregexp ^ba
/cap listworlds -s -msubstr a
/cap listworlds -s -msimple foo
/cap listworlds -s nomatch
/def lw = /echo n=$[listworlds("-s")]
/def lw2 = /test echo("n=" . listworlds("-s", "b*"))
/echo --info
/def info = \
  /echo [$[world_info("foo", "name")]][$[world_info("foo", "type")]][$[world_info("foo", "character")]][$[world_info("foo", "password")]]%; \
  /echo [$[world_info("foo", "host")]][$[world_info("foo", "port")]][$[world_info("baz", "mfile")]][$[world_info("baz", "src")]]%; \
  /echo [$[world_info("foo", "login")]][$[world_info("foo", "proxy")]][$[world_info("foo", "cipher")]]%; \
  /echo [$[world_info("nosuch", "name")]][$[world_info("name")]][$[world_info()]][$[world_info("Qux", "character")]][$[world_info("qux", "name")]]
/info
/echo --redef
/def redef = \
  /echo [$[addworld("foo", "tiny2")]]%; \
  /echo [$[world_info("foo", "type")]][$[world_info("foo", "host")]][$[world_info("foo", "character")]]%; \
  /echo [$[addworld("foo", "", "h3", "9")]]%; \
  /echo [$[world_info("foo", "host")]][$[world_info("foo", "port")]]%; \
  /echo [$[addworld("foo", "", "", "", "", "", "", "x")]]%; \
  /echo [$[addworld("foo", "", "", "", "", "", "", "ze")]]%; \
  /echo [$[addworld("a b", "")]][$[addworld("", "")]][$[addworld("x", "", "h", "")]][$[addworld("x", "", "", "9")]]%; \
  /echo [$[addworld("(x)", "", "h", "1")]][$[addworld("x2", "", "h", "1", "", "", "", "q")]]
/redef
/cap listworlds -c foo
/cap listworlds -c Qux
/echo --unworld
/def uw = \
  /echo [$[addworld("tmp1", "")]]%; \
  /unworld tmp1%; \
  /echo [$[world_info("tmp1", "name")]]
/uw
/cap unworld bar nosuch
/cap listworlds -s
/cap unworld default
/cap listworlds -s
/echo --rename
/cap renameworld foo foo2
/cap listworlds -s
/cap renameworld foo2 baz
/cap renameworld foo2 default
/cap renameworld foo2 (x)
/cap renameworld foo2 a
/cap renameworld nosuch b
/cap renameworld
/cap listworlds -s
/echo --state
/def st = \
  /echo [$[is_connected()]][$[is_connected("baz")]][$[is_open()]][$[is_open("baz")]][$[fg_world()]]%; \
  /echo [$[nactive()]][$[nactive("baz")]][$[idle("baz")]][$[sidle("baz")]][$[sidle()]][$[idle("nosuch")]]
/st
