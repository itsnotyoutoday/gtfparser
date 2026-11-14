;;; expand.317 # /@name that is not a builtin
/@nosuch
;;; expand.1810 # /@name that is not a builtin (in /eval)
/eval /@nosuch
;;; expand.253 # unknown command (hook NOMACRO text)
/nosuchcommand
;;; expand.510 # instruction count exceeded
/set max_instr=100
/def b1 = /while (1) /test 1%; /done
/b1
;;; expand.955 # recursion count exceeded
/def r1 = /r1
/r1
;;; expand.1300 # unexpected /done in an /if body
/def f = /if (1) /done%; /endif
/f
;;; expand.1303 # unexpected /endif in a /while body
/def f = /while (1) /endif%; /done
/f
;;; expand.1306 # unexpected /endif outside any block
/def f = /endif
/f
;;; expand.1360 # junk in an /if condition
/def f = /if (1 2) /echo x%; /endif
/f
;;; expand.1366 # %; right after an /if condition
/if (1)%; /echo hi%; /endif
;;; expand.1373 # text statement in an /if condition
/def f = /if 1 /echo x%; /endif
/f
;;; expand.1390 # /while without /done
/def f = /while (1) /echo x
/f
;;; expand.1418 # /break with a non-number
/def f = /while (1) /break x%; /done
/f
;;; expand.1469 # /if without /endif
/def f = /if (1) /echo x
/f
;;; expand.1506 # %; right after /else
/if (0) /echo a%; /else%; /endif
;;; expand.1566 # /return inside $() substitution
/def f = /echo $(/return 1)
/f
;;; expand.1597 # expression followed by junk in a macro
/def f = /test 1 2
/f
;;; expand.1627 # piping into a server command
/def f = /echo x %| hello
/f
;;; expand.1638 # piping out of a server command
/def f = hello %| /echo x
/f
;;; expand.1675 # %| with nothing after it
/def f = /echo hi %|
/f
;;; expand.1665 # missing %; between statements
/def f = /if (1) /echo x%; /endif /echo y
/f
;;; expand.1932 # unmatched $[
/eval /echo $[1+1
;;; expand.1965 # unmatched $(
/eval /echo $(/echo hi
;;; expand.2015 # unmatched ${
/eval /echo ${foo
;;; expand.2144 # illegal "-" in a substitution
/eval /echo %{-}
;;; expand.2204 # unmatched %{
/eval /echo %{foo
;;; expand.2205 # %{ with an illegal character
/eval /echo %{foo:}
;;; expand.2213 # %{x} inside an expression
/test 1 + %{x}
;;; expand.754 # ${name} of an undefined macro
/eval /echo ${nomacro}
;;; macro.860 # macro named like a builtin
/def undef = x
;;; macro.1770 # /trigger -n lists a named macro
/def -t"x" foo = /echo hi
/trigger -n x
;;; macro.1819 # /trigger -n summary
/def -t"x" foo = /echo hi
/trigger -n x
;;; world.188 # redefinition of a world
/test addworld("w", "", "h", "1")
/test addworld("w", "", "h", "2")
;;; variable.373 # local variable shadows a global
/set gv=1
/def f = /let gv=2
/f
;;; command.306 # /version, copyright line
/version
;;; command.307 # /version, contrib line
/version
;;; command.308 # /version, mods line
/version
;;; command.309 # /version, platform line
/version
;;; command.715 # /save under /restrict file
/restrict file
/save @DIR@/x
;;; command.736 # /load under /restrict file
/restrict file
/load @DIR@/x
;;; command.648 # a line that does not start with "/" in a loaded file
/test _h := tfopen("@DIR@/bad.tf", "w")
/test tfwrite(_h, "/echo a")
/test tfwrite(_h, "hello")
/test tfclose(_h)
/load @DIR@/bad.tf
;;; command.656 # loaded file ending in a backslash
/test _h := tfopen("@DIR@/bs.tf", "w")
/test tfwrite(_h, "/echo a \\")
/test tfclose(_h)
/load @DIR@/bs.tf
;;; history.673 # /log lists world logging
/test addworld("w", "", "h", "1")
/log -ww @DIR@/w.log
/log
