;;; command.128 # /substitute outside a trigger
/test substitute("x")
;;; command.246 # /reboot with an argument
/reboot foo
;;; command.270 # /sh under /restrict shell
/restrict shell
/sh true
;;; command.343 # /lcd under /restrict file
/restrict file
/lcd /tmp
;;; command.349 # /lcd to a nonexistent directory
/lcd @DIR@/nodir
;;; command.354 # /lcd prints the current directory
/lcd
;;; command.389 # echo() with an illegal destination
/test echo("x", "", "off", "z")
;;; command.424 # /restrict shows the level
/restrict
;;; command.610 # indented line after a command in a loaded file
/echo a
  /echo b
;;; command.625 # whitespace after a final backslash
/echo a\ 
/echo b
;;; command.852 # /unbind of an unbound key
/unbind ^Q^Q^Q
;;; command.305 # /version
/version
;;; process.338 # /kill of a nonexistent process
/kill 999999
;;; process.557 # /quote without a command character
/quote -1 x
;;; process.580 # /quote of a file under /restrict file
/restrict file
/quote -0 '@DIR@/x
;;; process.585 # /quote of a nonexistent file
/quote -0 '@DIR@/nofile
;;; process.648 # /repeat -S with a zero interval
/repeat -S i /echo x
;;; process.657 # /repeat with a negative count
/repeat -1 0 /echo x
;;; process.661 # /repeat with a non-numeric count
/repeat -1 x /echo x
;;; process.665 # /repeat count followed by garbage
/repeat -1 3x /echo x
;;; process.670 # /repeat without a command
/repeat -1 3
;;; history.349 # /recall with no arguments
/recall
;;; history.373 # /recall with a number that is too large
/recall 99999999999999999999
;;; history.395 # /recall range end too large
/recall 1-99999999999999999999
;;; history.412 # extra text after a /recall range
/recall 5x
;;; history.719 # two history-selecting options
/histsize -i -g 10
;;; history.777 # /log under /restrict file
/restrict file
/log @DIR@/x
;;; history.834 # /log to an unwritable path
/log @DIR@/nodir/x
;;; history.843 # /log to a file reports it
/log @DIR@/log.txt
;;; keyboard.89 # key binding that extends an existing binding
/def -b"^A" = /echo x
/def -b"^A^B" = /echo y
;;; keyboard.326 # /dokey with an unknown editing function
/dokey nosuchfunc
;;; attr.388 # invalid display attribute
/echo -aZ hi
;;; attr.638 # unmatched @{
/echo -p @{u
;;; socket.2628 # /dc with no current socket
/dc
;;; socket.2640 # /dc of an unconnected world
/dc nosuchworld
;;; socket.2730 # /listsockets with a bad sort criterion
/listsockets -Sbogus
;;; socket.2744 # /listsockets with no sockets
/listsockets
;;; socket.2866 # send() with an invalid flag
/test send("x", "", "Z")
;;; socket.2955 # fake_recv() with an invalid flag
/test fake_recv("x", "", "Z")
;;; socket.2964 # fake_recv() to an unknown world
/test fake_recv("x", "nosuchworld")
;;; socket.3052 # sending text with no connection
/def f = hello
/f
;;; socket.4471 # /connect to an unknown world
/echo -wnosuch hi
;;; socket.1361 # /fg of an unknown socket
/fg nosuchworld
;;; socket.1394 # /connect to an arbitrary host under /restrict world
/restrict world
/connect somehost 1234
;;; pattern.155 # regexp containing "{"
/test regmatch("a{2}", "aa")
;;; pattern.199 # bad regexp
/test regmatch("(", "x")
;;; pattern.546 # unmatched "[" in a glob
/test "a" =/ "[a"
;;; pattern.553 # nested "{" in a glob
/test "a" =/ "{a{b}}"
;;; pattern.558 # char before "{" in a glob
/test "a" =/ "a{b}"
;;; pattern.566 # char after "}" in a glob
/test "a" =/ "{b}c"
;;; pattern.576 # space inside "{...}"
/test "a" =/ "{a b}"
;;; pattern.583 # unmatched "{" in a glob
/test "a" =/ "{a"
;;; macro.729 # redefining a key binding
/def -b"^A" = /echo x
