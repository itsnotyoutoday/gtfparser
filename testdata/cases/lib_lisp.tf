/require lisp.tf
/def r_car = /echo [$(/car %*)]
/def q_car = /let _r=$(/car %*)%; /echo {%{_r}}
/r_car a b c
/r_car 
/r_car one
/r_car "q r" s
/r_car   lead x
/r_car 1 2 3
/q_car a b c
/def r_cdr = /echo [$(/cdr %*)]
/def q_cdr = /let _r=$(/cdr %*)%; /echo {%{_r}}
/r_cdr a b c
/r_cdr a
/r_cdr 
/r_cdr x  y  z
/r_cdr 1 2
/q_cdr a b c
/def r_cadr = /echo [$(/cadr %*)]
/def q_cadr = /let _r=$(/cadr %*)%; /echo {%{_r}}
/r_cadr a b c
/r_cadr a
/r_cadr 
/q_cadr a b c
/def r_cddr = /echo [$(/cddr %*)]
/def q_cddr = /let _r=$(/cddr %*)%; /echo {%{_r}}
/r_cddr a b c d
/r_cddr a b
/r_cddr a
/q_cddr a b c d
/def r_caddr = /echo [$(/caddr %*)]
/def q_caddr = /let _r=$(/caddr %*)%; /echo {%{_r}}
/r_caddr a b c d
/r_caddr a b
/q_caddr a b c d
/def r_cdddr = /echo [$(/cdddr %*)]
/def q_cdddr = /let _r=$(/cdddr %*)%; /echo {%{_r}}
/r_cdddr a b c d e
/r_cdddr a b c
/q_cdddr a b c d e
/def r_length = /echo [$(/length %*)]
/def q_length = /let _r=$(/length %*)%; /echo {%{_r}}
/r_length a b c d
/r_length 
/r_length    x   y   
/r_length "a b" c
/r_length 1
/q_length a b c d
/def r_reverse = /echo [$(/reverse %*)]
/def q_reverse = /let _r=$(/reverse %*)%; /echo {%{_r}}
/r_reverse a b c d
/r_reverse a
/r_reverse 1 2 3 4 5 6
/r_reverse "a b" c
/r_reverse 
/q_reverse a b c d
/def r_remove = /echo [$(/remove %*)]
/def q_remove = /let _r=$(/remove %*)%; /echo {%{_r}}
/r_remove b a b c b d
/r_remove x a b c
/r_remove a a
/r_remove a a a a
/r_remove 1 1 2 1 3
/r_remove a
/q_remove b a b c b d
/def r_unique = /echo [$(/unique %*)]
/def q_unique = /let _r=$(/unique %*)%; /echo {%{_r}}
/r_unique a b a c b d
/r_unique a a a
/r_unique a
/r_unique 1 2 3 2 1
/r_unique a b c
/r_unique x y x y x
/q_unique a b a c b d
/def show = /echo item:%1
/mapcar show a b c
/mapcar show
/mapcar show 1
/mapcar show "a b" c
/maplist show a b c
/def pr = /echo pair:%*
/maplist pr a b c d
/maplist pr x
/maplist pr
/test E(car("a b", "c"))
/test E(cdr(1, 2, 3))
/test E(length(1, 2, 3, 4))
/test E(length())
/test E(reverse(1, 2, 3))
/test E(remove("b", "a", "b", "c"))
/test E(unique(1, 1, 2, 2, 3))
/test E(cadr(1, 2, 3))
/test E(caddr(1, 2, 3))
/test E(cddr(1, 2, 3, 4))
/test E(cdddr(1, 2, 3, 4, 5))
/test E(car(reverse(1, 2, 3)))
/test E(strlen(reverse("ab", "cd")))
/r_car $(/reverse a b c)
/r_length $(/unique a b a b c)
/r_reverse $(/cdr $(/reverse a b c d))
/r_unique $(/reverse x y x z)
/def lsum = /let s=0%; /while (shift(), {#}) /test s+=%1%; /done%; /result s
/test E(lsum(0, 1, 2, 3, 4))
/def sort2 = /let r=$(/reverse %*)%; /echo %r
/sort2 3 2 1
