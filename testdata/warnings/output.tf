;;; output.1328 # unterminated quote in a status field spec
/status_add "abc
;;; output.1364 # garbage after a status field spec
/status_add "abc"x
;;; output.1504 # /status_rm of a field that does not exist
/status_rm nosuchfield
;;; output.1504.b # /status_rm of a field that does not exist, in a row
/status_rm -r2 nosuchfield
;;; output.1526 # status_int_* changed default
/set status_int_clock=5
;;; output.1595 # two variable-width status fields
/status_add -A @more
;;; output.1613 # status wider than the screen
/status_add -r1 abcdefghij:100000
;;; output.1717 # setting status_fields directly
/set status_fields=abc
;;; output.2167 # status_height above the maximum
/set status_height=100
;;; output.2178 # expnonvis on a terminal without support
/set expnonvis=on
;;; output.3200 # keypad on a terminal that cannot
/set keypad=on
;;; help.102 # /help for an unknown topic
/help nosuchtopic
;;; history.567 # /watchdog shows its state
/watchdog
;;; history.572 # /watchdog off
/watchdog off
;;; history.583 # /watchdog enabled with a window
/watchdog 1 3
;;; history.790 # /log lists input logging
/log -i @DIR@/i.log
/log
;;; history.792 # /log lists local logging
/log -l @DIR@/l.log
/log
;;; history.795 # /log lists global logging
/log -g @DIR@/g.log
/log
;;; history.874 # /histsize shows capacity
/histsize
;;; macro.1468 # /save reports the file
/save @DIR@/m.tf
;;; world.563 # /saveworld reports the file
/saveworld @DIR@/w.tf
;;; macro.1758 # /trigger -n lists the macros it would run
/def -t"x" foo = /echo hi
/trigger -n x
;;; variable.665 # /set of an unset variable
/set zzz
;;; variable.669 # /set shows a value
/set zzz=5
/set zzz
;;; command.828 # /hook shows state
/hook
;;; squat.1901 # /squat status
/squat
;;; squat.1706 # /squat off when off
/squat off
;;; squat.1918 # /squat subcommand needing squat on
/squat pair
;;; squat.1975 # /control allow without a request
/control allow
;;; squat.2013 # /control for an unknown world
/control nosuchworld tf
