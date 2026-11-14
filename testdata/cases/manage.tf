/def capl = /list %* %| /while (tfread(l) >= 0) /echo [%l]%; /done
/def capv = /listvar %* %| /while (tfread(l) >= 0) /echo [%l]%; /done
/def capd = /def %* %| /while (tfread(l) >= 0) /echo [%l]%; /done
/def foo = /echo hi %1
/def -t'abc*' -p3 -n2 bar = /echo bar
/def -mregexp -t'x(y)z' -F -aB baz = /echo baz
/def -h'CONNECT w*' hk = /echo hk
/def -i invis = /echo hidden
/def quux
/capl foo
/capl -s foo
/capl bar
/capl baz
/capl hk
/capl invis
/capl -i invis
/capv -s sigfigs
/capv borg
/set myvar=hello world
/capv myvar
/capv -s myv*
/undef foo
/capl foo
/test E(ismacro("bar") > 0)
/test E(isvar("myvar"))
/test E(isvar("nosuchvar"))
/capd foo = /echo again
/capl foo
