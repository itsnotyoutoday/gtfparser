/repeat -S 3 /echo hi
/def r2 = /repeat -S 2 /echo [%%1]
/r2 x
/quote -S -decho !echo one; echo two
/quote -S -decho !printf 'a\nb\nc\n'
/quote -S -dsend !echo sent
/quote -S /echo pre: !echo xx
/quote -S -dexec !echo /echo exec: yy
