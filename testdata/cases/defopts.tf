/def capl = /list %* %| /while (tfread(l) >= 0) /echo [%l]%; /done
/echo -- option clusters and quoting
/def -Fp3 -t'a b' o1 = x
/capl o1
/def -t"it's" -p2 o2
/capl o2
/def -t'it\'s \\ back' o3
/capl o3
/def -t`bq` o4
/capl o4
/def -t'q'p4 o5
/capl o5
/def -t"x"=y o6
/capl o6
/def -ip2 -t'inv' o7
/capl -i o7
/def -mregexp -n3 -t'n3' o8
/capl o8
/def -1 -t'n1' o9
/capl o9
/def -n0 -t'n0' o10
/capl o10
/def -n-5 -t'nneg' o11
/capl o11
/echo -- numeric option expressions
/set pv=6
/def -p$[pv+1] -t'pe' o12
/capl o12
/def -p%{pv} -t'pe2' o13
/capl o13
/def -c$[2*30] -t'pc' o14
/capl o14
/echo -- end of options
/def -t'a' -- o15
/capl o15
/def -t'a' - o16 = x
/capl o16
/def -- -o17 = x
/capl -- -o17
/def -F -p1 -- o18 = body
/capl o18
/echo -- names
/def name with = spaces
/def   spaced   = body
/capl spaced
/def tab	body = x
/echo -- body whitespace
/def sp1 =    leading spaces
/capl sp1
/def sp2 = trailing
/capl sp2
/def sp3 =
/capl sp3
/def sp4 = = eq
/capl sp4
/def sp5=nospace
/capl sp5
/def sp6= x
/capl sp6
/echo -- redefinition keeps being listed by newest number
/def rn1 = a
/def rn1 = b
/capl rn1
/echo -- redef off
/set redef=off
/def rn1 = c
/capl rn1
/def rn1 = b
/def rn2 = z
/def rn2 = y
/capl rn2
/set redef=on
/def rn2 = y
/capl rn2
/echo -- list selection by options
/def -t'la' -p2 -n3 -aB l1 = body one
/def -t'lb' -p3 -F l2 = body two
/def -hCONNECT -p2 l3 = body three
/def -h'CONNECT x*' -c20 l4 = body four
/def -t'lc' -w'' -T'foo' l5 = body five
/def -b'^Xa' l6 = body six
/def -E'1' -q l7 = body seven
/def -w l8 = body eight
/def -i l9 = nine
/def l10 = body ten
/capl l1
/capl -s l1
/capl -t'la' l[1-2]
/capl -p2 l1
/capl -p2 l2
/capl -n3 l1
/capl -n l1
/capl -F l2
/capl -F l1
/capl -c20 l4
/capl -c100 l4
/capl -E l7
/capl -E'1' l7
/capl -E'2' l7
/capl -b l6
/capl -b'^Xa' l6
/capl -b'^Xb' l6
/capl -T l5
/capl -T'foo' l5
/capl -T'bar' l5
/capl -t l1
/capl -t l10
/capl -t'{}' l10
/capl -t'{}' l1
/capl -mregexp -t'^$' l10
/capl -mregexp -t'^l' l1
/capl -msubstr -t'a' l1
/capl -msimple -t'la' l1
/capl -msimple -t'l' l1
/capl -h l3
/capl -h l1
/capl -h0 l1
/capl -h0 l3
/capl -hCONNECT l3
/capl -hCONNECT l4
/capl -hWORLD l3
/capl -h'CONNECT x*' l4
/capl -h'CONNECT y*' l4
/capl -h'CONNECT|WORLD' l4
/capl l9
/capl -i l9
/capl -I l9
/capl -I l10
/capl -i l10
/capl -msimple l10
/capl l1 = body one
/capl l[1-3] = body one
/capl -mregexp ^l[1-2]$ = ^body o
/capl -mregexp ^l1$ = ^body
/capl -msimple l1 = body one
/capl -msimple l1 = body
/capl -mregexp l7 = {}
/capl -mglob l7 = {}
/capl -mglob l6 = body*
/capl -mglob l6 = nomatch*
/capl -a l1
/capl -an l10
/capl -ag l1
/capl -w l8
/capl -w- l10
/capl -w- l8
/capl #1
/echo -- -S sorts by name
/def sa = 1
/def sc = 1
/def sb = 1
/def -i sorted = /list -s -S s[abc] %| /echo %%; /test 1
/echo -- list returns the last number
/def lr1 = x
/def lr2 = x
/test E(1)
/echo -- list of nothing
/capl nosuchmacro
/capl #99999
