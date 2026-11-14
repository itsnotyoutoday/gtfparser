;;; expr.386 # mod(1e300,1): real too large for an integer argument
/test mod(1e300, 1)
;;; expr.413 # ftime() with a real time argument too large
/test ftime("%H", 1e300)
;;; expr.542 # expression nested deeper than the 512-slot value stack
/test 1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1+(1))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))
;;; expr.621 # float overflow in a binary operator
/test 1e308 * 10
;;; expr.650 # float overflow in an assignment operator
/test z := 1e308
/test z *= 10
;;; expr.709 # assignment to something that is not a variable
/test 1 := 2
;;; expr.755 # arithmetic on two absolute times
/test time() + time()
;;; expr.762 # unary minus on an absolute time
/test -time()
;;; expr.773 # regmatch() compared to 1
/test regmatch("a", "a") == 1
;;; expr.813 # division by zero
/test 1/0
;;; expr.917 # addworld() under /restrict world
/restrict world
/test addworld("w", "x", "h", "1")
;;; expr.934 # addworld() with an unknown flag letter
/test addworld("w", "x", "h", "1", "", "", "", "Z")
;;; expr.1089 # fwrite() to an unwritable path
/test fwrite("@DIR@/nodir/x", "a")
;;; expr.1107 # tfclose("e")
/test tfclose("e")
;;; expr.1130 # tfread() into a non-variable
/test tfread(1)
;;; expr.1168 # keycode() of an unknown key name
/test keycode("zzzz")
;;; expr.1173 # mod() by zero
/test mod(1, 0)
;;; expr.1199 # moresize() with an illegal flag
/test moresize("x")
;;; expr.1355 # world_info() with an unknown field name
/test addworld("w", "", "h", "1")
/test world_info("w", "nofield")
;;; expr.1604 # getopts() outside a command
/test getopts("a")
;;; expr.1613 # getopts() with an invalid option specifier
/def f = /test getopts("1a", "")
/f -a
;;; expr.1657 # deprecated read()
/test read()
;;; expr.1757 # call of an undefined function
/test nofunc(1)
;;; expr.1904 # "=" under /set pedantic
/set pedantic=on
/test 1 = 1
;;; expr.1985 # call of a non-identifier
/test 1(2)
;;; expr.1995 # exit() is not a function
/test exit(1)
;;; expr.2015 # too few arguments to a builtin function (exact count)
/test strlen()
;;; expr.2015.b # too few arguments to a builtin function (range)
/test substr("a")
;;; expr.2022 # command called as a function with 2 arguments
/test version(1, 2)
;;; expr.2049 # unterminated string literal
/test "abc
;;; expr.2066 # statement start after an expression
/test 1 /echo hi
;;; expr.2072 # $[...] inside an expression
/test $[1]
;;; expr.1841 # ?: without the colon
/test 1 ? 2
;;; expr.2007 # function arguments not separated by commas
/test strlen(1 2)
;;; expr.2085 # missing ")" after "(..."
/test (1 2)
;;; expr.2090 # missing operand
/test 1 +
;;; expand.393 # trailing junk after an expression
/test 1 2
;;; expand.1934 # $[ ] with trailing junk
/eval /echo $[1 2]
