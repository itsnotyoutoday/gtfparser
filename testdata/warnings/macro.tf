;;; macro.161 # /def -h with the misspelled legacy event CONNETFAIL
/def -hCONNETFAIL = /echo x
;;; macro.163 # /def -h with an unknown hook event
/def -hBOGUS = /echo x
;;; macro.255 # two different -m options
/def -mglob -mregexp foo = x
;;; macro.282 # deprecated /def -B
/def -Bx foo = x
;;; macro.305 # /def -w with an unknown world
/def -w"nosuchworld" foo = x
;;; macro.332 # -P given twice
/def -mregexp -t"a(b)" -P0Cred -P1Cblue foo = x
;;; macro.335 # -P with a non-regexp -m
/def -mglob -P0Cred foo = x
;;; macro.371 # negative -P subexpression number
/def -mregexp -t"a(b)" -P-1Cred foo = x
;;; macro.650 # /edit $text of a hook that does not exist
/edit $nothing
;;; macro.984 # /edit with no macro name
/edit -hCONNECT
;;; macro.732 # key binding already exists with redef off
/def -b"^A" = /echo x
/set redef=off
/def -b"^A" = /echo y
;;; macro.782 # unnamed trigger under pedantic
/set pedantic=on
/def -t"x" = /echo hi
;;; macro.846 # illegal macro name
/def #foo = x
;;; macro.854 # reserved word as macro name
/def if = x
;;; macro.900 # -P without -mregexp (at completion)
/def -P0Cred foo = x
;;; macro.911 # -P subexpression beyond the pattern's groups
/def -mregexp -t"a" -P1Cred foo = x
;;; macro.924 # key name that does not exist
/def -Bnokeyname = x
;;; macro.930 # key name with no code
/def -BF9 = x
;;; macro.943 # redefinition with redef off
/def foo = 1
/set redef=off
/def foo = 2
;;; macro.991 # /edit of an undefined macro
/edit nomacro = x
;;; macro.1134 # /undef of an undefined macro
/undef nomacro
;;; macro.1198 # /undef of a macro number that does not exist
/undef #9999
;;; macro.1463 # /save to an unwritable path
/save @DIR@/nodir/x.tf
;;; macro.1920 # trigger rate limit
/set max_trig=1
/def -t"zzz" = /echo hi
/trigger zzz
/trigger zzz
/trigger zzz
/trigger zzz
/set max_hook=1
/def -hNOMACRO = /echo x
/foo1
/foo2
/foo3
/foo4
