/def r = /test E(regmatch({1}, {2})) %; /echo [%{P0}] [%{P1}] [%{P2}] [%{PL}] [%{PR}]
/test E(regmatch("(a+)(b*)", "xaabbby"))
/echo [%{P0}] [%{P1}] [%{P2}] [%{PL}] [%{PR}]
/test E(regmatch("(\\w+) \\1", "hello hello world"))
/echo [%{P0}] [%{P1}]
/test E(regmatch("foo(?=bar)", "foobar"))
/echo [%{P0}] [%{PR}]
/test E(regmatch("(?<=a)b", "ab"))
/echo [%{P0}] [%{PL}]
/test E(regmatch("a++b", "aaab"))
/test E(regmatch("(?>a+)ab", "aaab"))
/test E(regmatch("^abc$", "abc"))
/test E(regmatch("abc$", "abc"))
/test E(regmatch("HELLO", "hello"))
/test E(regmatch("hello", "HELLO"))
/test E(regmatch("(?i)HELLO", "hello"))
/test E(regmatch("é", "É"))
/test E(regmatch("a.b", "a b"))
/test E(regmatch("a\\.b", "axb"))
/test E(regmatch("[[:digit:]]+", "ab123cd"))
/echo [%{P0}]
/test E(regmatch("(a)|(b)", "b"))
/echo [%{P0}] [%{P1}] [%{P2}]
/test E(regmatch("x*", "abc"))
/echo [%{P0}] [%{PL}] [%{PR}]
/test E(regmatch("\\bfoo\\b", "a foo b"))
/test E(regmatch("a{2,3}", "aaaa"))
/echo [%{P0}]
/test E(regmatch("a{2,3}?", "aaaa"))
/echo [%{P0}]
/test E(regmatch("(?P<n>x)y", "xy"))
/echo [%{P1}]
