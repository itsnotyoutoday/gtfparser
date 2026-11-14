;;; world.111 # illegal world name
/test addworld("a b", "x", "h", "1")
;;; world.135 # host without port
/test addworld("w", "x", "h", "")
;;; world.370 # /unworld of an unknown world
/unworld nosuchworld
;;; world.426 # /listworlds with a bad sort criterion
/listworlds -Sbogus
;;; world.548 # /saveworld under /restrict file
/restrict file
/saveworld
;;; world.560 # /saveworld to an unwritable path
/saveworld @DIR@/nodir/worlds
;;; world.248 # /renameworld without arguments
/renameworld
;;; world.253 # /renameworld with one argument and no connection
/renameworld foo
;;; world.258 # /renameworld of an unknown world
/renameworld nosuch other
;;; world.597 # illegal charset name
/set default_charset=nosuchcharset
;;; world.181 # world definition with a password in a world-readable file
/addworld w x h 1 c pw
