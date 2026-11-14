;;; variable.300 # variable reference that looks like a special substitution
/test R
;;; variable.358 # variable named like a special substitution
/set L=1
;;; variable.609 # '=' after a space in /set
/set foo =bar
;;; variable.657 # illegal variable name
/set a-b=1
;;; variable.730 # /export of an undefined variable
/export nosuchvar
;;; variable.745 # /unset of a positive-integer variable
/unset histsize
;;; variable.861 # integer variable below its minimum
/set histsize=-1
;;; variable.972 # obsolete variable
/set prompt_sec=1
;;; variable.978 # undocumented variable
/set virtscreen=off
;;; util.219 # invalid enumerated value (string)
/set scroll=bogus
;;; util.221 # invalid enumerated value (number)
/set emulation=77
;;; util.312 # missing numeric argument
/kill x
;;; util.352 # unknown escape inside a quoted string under pedantic
/set pedantic=on
/def -t"a\x" foo
;;; util.446 # bad option syntax
/def --x foo = bar
;;; util.477 # invalid option
/def -Z foo = bar
;;; util.487 # option summary after an invalid option
/def -? foo = bar
;;; util.499 # bad quoted option argument
/def -t"abc foo = bar
;;; util.526 # bad time option
/repeat -S 1 1 x
;;; util.564 # locale category set
/set LC_CTYPE=C
;;; util.827 # integer literal too large
/recall 99999999999999999999
;;; util.905 # time value too large
/test 99999999999999999999:00:00
;;; util.917 # numeric value too large
/test 1e999
