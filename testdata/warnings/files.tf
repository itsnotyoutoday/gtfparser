;;; command.719 # /save with no filename and no default macro
/save
;;; command.749 # /load with no filename
/load
;;; command.576 # /load of a file that does not exist
/load @DIR@/nofile.tf
;;; command.580 # /load prints "Loading commands from"
/def -i helper = /echo ok
/test _h := tfopen("@DIR@/inc.tf", "w")
/test tfwrite(_h, "/echo included")
/test tfclose(_h)
/load @DIR@/inc.tf
;;; tfio.102 # missing filename and default macro undefined
/undef WORLDFILE
/saveworld
;;; tfio.926 # tfopen() under /restrict file
/restrict file
/test tfopen("@DIR@/z", "w")
;;; tfio.931 # tfopen() with an invalid mode
/test tfopen("@DIR@/z", "x")
;;; tfio.936 # tfopen() of an unopenable file
/test tfopen("@DIR@/nodir/z", "w")
;;; tfio.965 # tfread() on a bad handle
/test tfread("nosuchhandle", x)
;;; tfio.980 # tfwrite() on a read-only stream
/test tfwrite("i", "x")
;;; tfio.850 # read from keyboard in non-command-line context
/test tfread("i", x)
