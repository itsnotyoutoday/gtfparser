;;; ctx.file-line # error at file level: "<file>, line N: "
/echo a
/echo b
/test nofunc(1)
;;; ctx.macro # error inside a macro: the macro name follows the location
/def f = /test nofunc(1)
/f
;;; ctx.macro-nested # error in a macro called from a macro
/def inner = /test nofunc(1)
/def outer = /inner
/outer
;;; ctx.multiline # command spread over lines with backslash continuation: "lines N-M: "
/echo a
/test \
  nofunc(1)
;;; ctx.eval # error in /eval
/eval /test nofunc(1)
;;; ctx.trigger # error in a trigger macro
/def -t"zzz" trg = /test nofunc(1)
/trigger zzz
;;; ctx.hook # error in a hook macro
/def -hNOMACRO = /test nofunc(1)
/zzz
;;; ctx.load-nested # error in a file loaded from a file: the inner file is named
/test _h := tfopen("@DIR@/inner.tf", "w")
/test tfwrite(_h, "/echo x")
/test tfwrite(_h, "/test nofunc(1)")
/test tfclose(_h)
/load @DIR@/inner.tf
;;; ctx.option-suffix # option parse error: command name carries the option letter
/def -Z foo = bar
;;; ctx.function-name # error raised by a function: function name is the command
/test mod(1e300, 1)
;;; ctx.cmdsub # error inside $() command substitution
/def n2 = /test nofunc(1)
/eval /echo $(/n2)
;;; ctx.builtin-in-macro # builtin command error inside a macro: builtin name replaces the macro name
/def g = /undef nope
/g
;;; ctx.shadowed-builtin # macro shadowing a builtin name
/def -i undef = /echo shadow
/test nofunc(1)
;;; ctx.nonexistent-command # unknown command (hook NOMACRO text)
/nosuchcommand
;;; ctx.nonexistent-in-macro # unknown command inside a macro
/def h = /nosuchcommand
/h
;;; ctx.load-error # /load of a missing file
/load @DIR@/missing.tf
;;; ctx.load-implicit-dir # /load of a directory
/load @DIR@
;;; ctx.eval-macro # macro invoked through /eval with arguments
/def a1 = /test nofunc(1)
/eval /a1 x y
;;; ctx.if-condition # error in an /if condition inside a macro
/def c1 = /if (nofunc(1)) /echo x%; /endif
/c1
;;; ctx.while-body # error in a /while body inside a macro
/def w1 = /while (nofunc(1)) /echo x%; /done
/w1
