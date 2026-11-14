;;; ctx.typed typed
/test nofunc(1)
;;; ctx.typed-macro typed
/def f = /test nofunc(1)
/f
;;; ctx.typed-builtin typed
/undef nosuchmacro
;;; ctx.typed-unknown typed
/nosuchcommand
;;; ctx.typed-option typed
/def -Z foo = bar
