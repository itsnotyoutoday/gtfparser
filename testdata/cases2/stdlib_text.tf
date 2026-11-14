/def r_first = /echo [$(/first %*)]
/def q_first = /let _r=$(/first %*)%; /echo {%{_r}}
/r_first a b c
/r_first 
/r_first one
/r_first "q r" s
/r_first   lead x
/r_first 1 2 3
/r_first a  b
/q_first a b c
/def r_rest = /echo [$(/rest %*)]
/def q_rest = /let _r=$(/rest %*)%; /echo {%{_r}}
/r_rest a b c
/r_rest a
/r_rest 
/r_rest 1 2 3 4
/r_rest   x   y  z 
/q_rest a b c
/def r_last = /echo [$(/last %*)]
/def q_last = /let _r=$(/last %*)%; /echo {%{_r}}
/r_last a b c
/r_last a
/r_last 
/r_last x  y  
/r_last 1 2 3
/q_last a b c
/def r_nth = /echo [$(/nth %*)]
/def q_nth = /let _r=$(/nth %*)%; /echo {%{_r}}
/r_nth 1 a b c
/r_nth 2 a b c
/r_nth 3 a b c
/r_nth 4 a b
/r_nth 0 a b
/r_nth -1 a b
/r_nth 1
/r_nth 2 a
/r_nth 10 1 2 3
/r_nth 2 "x y" z
/q_nth 1 a b c
/def r_escape = /echo [$(/escape %*)]
/def q_escape = /let _r=$(/escape %*)%; /echo {%{_r}}
/r_escape * a*b*c
/r_escape () (a) (b)
/r_escape . x.y.z
/r_escape / a/b/c
/r_escape . 
/r_escape x abc
/r_escape "" abc
/r_escape [] a[1]b
/r_escape * 
/r_escape \\ a\b
/q_escape * a*b*c
/def r_replace = /echo [$(/replace %*)]
/def q_replace = /let _r=$(/replace %*)%; /echo {%{_r}}
/r_replace a X banana
/r_replace an AN banana
/r_replace xyz Q banana
/r_replace b "" abbc
/r_replace - . a-b-c
/r_replace a bb aaa
/r_replace a b
/r_replace a b 
/r_replace l L hello world
/r_replace . ! a.b.c
/q_replace a X banana
/def r_expr = /echo [$(/expr %*)]
/def q_expr = /let _r=$(/expr %*)%; /echo {%{_r}}
/r_expr 1+2
/r_expr 3 * 4
/r_expr 10 / 3
/r_expr 10.0 / 4
/r_expr strlen("abc")
/r_expr "a" =~ "a"
/r_expr 5 > 3
/r_expr 1,2,3
/r_expr "x" ? 1 : 2
/r_expr -3 + 1
/r_expr 
/q_expr 1+2
/def r_textencode = /echo [$(/textencode %*)]
/def q_textencode = /let _r=$(/textencode %*)%; /echo {%{_r}}
/r_textencode hello world
/r_textencode a_b
/r_textencode 
/r_textencode x.y/z
/r_textencode abc123
/r_textencode !@#
/r_textencode Hello, World!
/q_textencode hello world
/def r_textdecode = /echo [$(/textdecode %*)]
/def q_textdecode = /let _r=$(/textdecode %*)%; /echo {%{_r}}
/r_textdecode hello_32_world
/r_textdecode a_b
/r_textdecode _65__66_
/r_textdecode x_46_y
/r_textdecode no_more_here_
/r_textdecode _300_
/q_textdecode hello_32_world
/def r_tr = /echo [$(/tr %*)]
/def q_tr = /let _r=$(/tr %*)%; /echo {%{_r}}
/r_tr abc xyz aabbcc
/r_tr OIS. 01Z! TINYFUGUE.IS.COOL
/r_tr a b abc
/r_tr abc xy abcabc
/r_tr x y
/r_tr aeiou AEIOU the quick brown fox
/r_tr abc ABC
/q_tr abc xyz aabbcc
/test E(vercmp("5.0 beta 8", "5.0 beta 8"))
/test E(vercmp("5.0 beta 8", "5.0 beta 9"))
/test E(vercmp("5.0 beta 10", "5.0 beta 9"))
/test E(vercmp("5.0 alpha 8", "5.0 beta 1"))
/test E(vercmp("5.1 alpha 8", "5.0 stable 1"))
/test E(vercmp("4.9 stable 0", "5.0 alpha 1"))
/test E(vercmp("5.0 gamma 1", "5.0 stable 1"))
/test E(vercmp("bogus", "5.0 beta 1"))
/test E(vercmp("5.0 beta 1", "bogus"))
/test E(vercmp("10.0 beta 1", "9.9 stable 5"))
/def sp = /split %*%; /echo [%P1][%P2]
/sp name = value
/sp name=value
/sp x
/sp   a b  =  c d
/sp =v
/sp a=
/sp
/sp k = "v w"
/sp a = b = c
/sp a b c
/test split("n = v")
/test E(strcat(split("n = v"), "|", {P1}, "|", {P2}))
/test E(strcat(split(""), "|", {P1}, "|", {P2}))
/test E(escape("*", "a*b"))
/test E(replace("a", "b", "banana"))
/test E(first(1, 2, 3))
/test E(rest(1, 2, 3))
/test E(last(1, 2, 3))
/test E(nth(2, "a", "b", "c"))
/test E(nth(0, "a"))
/test E(nth(5, "a"))
/test E(expr(1+1))
/test E(tr("abc", "xyz", "aabbcc"))
/test E(textdecode(textencode("a b-c!")))
/test E(textencode("hello world"))
/def trip = /echo $(/textdecode $(/textencode %*))
/trip hello, world!
/trip   multiple   spaces
/trip %percent $dollar \backslash
/trip a_b_c
