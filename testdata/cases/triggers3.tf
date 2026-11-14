/echo -- patterns
/def -F -p1 -mregexp -t'foo' r1 = /echo %{0} re 1
/def -F -p2 -mregexp -t'Foo' r2 = /echo %{0} re 2
/def -F -p3 -mregexp -t'^foo' r3 = /echo %{0} re 3
/def -F -p4 -mregexp -t'^Foo' r4 = /echo %{0} re 4
/def -F -p5 -mregexp -t'foo$' r5 = /echo %{0} re 5
/def -F -p6 -mregexp -t'^foo$' r6 = /echo %{0} re 6
/def -F -p7 -mregexp -t'fo+' r7 = /echo %{0} re 7
/def -F -p8 -mregexp -t'fo*bar' r8 = /echo %{0} re 8
/def -F -p9 -mregexp -t'f(o)o' r9 = /echo %{0} re 9
/def -F -p10 -mregexp -t'foo|bar' r10 = /echo %{0} re 10
/def -F -p11 -mregexp -t'(?i)FOO' r11 = /echo %{0} re 11
/def -F -p12 -mregexp -t'^ab+c' r12 = /echo %{0} re 12
/def -F -p13 -mregexp -t'a.c' r13 = /echo %{0} re 13
/def -F -p14 -mregexp -t'\\bfoo' r14 = /echo %{0} re 14
/def -F -p15 -mregexp -t'x{2}' r15 = /echo %{0} re 15
/def -F -p16 -mregexp -t'[fF]oo' r16 = /echo %{0} re 16
/def -F -p17 -mregexp -t'foo?' r17 = /echo %{0} re 17
/def -F -p18 -mregexp -t'foo bar' r18 = /echo %{0} re 18
/def -F -p19 -mregexp -t'^foo bar' r19 = /echo %{0} re 19
/def -F -p20 -mregexp -t'café' r20 = /echo %{0} re 20
/def -F -p21 -mregexp -t'k' r21 = /echo %{0} re 21
/def -F -p22 -mregexp -t'ſ' r22 = /echo %{0} re 22
/def -F -p23 -mregexp -t's' r23 = /echo %{0} re 23
/def -F -p24 -mregexp -t'^s' r24 = /echo %{0} re 24
/def -F -p25 -mregexp -t'K' r25 = /echo %{0} re 25
/def -F -p26 -mregexp -t'^k' r26 = /echo %{0} re 26
/def -F -p27 -mregexp -t'kel' r27 = /echo %{0} re 27
/def -F -p28 -mregexp -t'^fo{1}' r28 = /echo %{0} re 28
/def -F -p29 -mregexp -t'fo{1,2}x' r29 = /echo %{0} re 29
/def -F -p30 -mregexp -t'foo.*' r30 = /echo %{0} re 30
/def -F -p31 -mregexp -t'.*foo' r31 = /echo %{0} re 31
/def -F -p32 -mregexp -t'(foo)' r32 = /echo %{0} re 32
/def -F -p33 -mregexp -t'(?-i)foo' r33 = /echo %{0} re 33
/def -F -p34 -mregexp -t'^' r34 = /echo %{0} re 34
/def -F -p35 -mregexp -t'$' r35 = /echo %{0} re 35
/def -F -p36 -mregexp -t'' r36 = /echo %{0} re 36
/def -F -p37 -mregexp -t'a|b' r37 = /echo %{0} re 37
/def -F -p38 -mregexp -t'^a|b' r38 = /echo %{0} re 38
/def -F -p39 -mregexp -t'foo(bar)?' r39 = /echo %{0} re 39
/def -F -p40 -mregexp -t'o' r40 = /echo %{0} re 40
/def -F -p41 -mregexp -t'oo' r41 = /echo %{0} re 41
/def -F -p42 -mregexp -t'\\w+' r42 = /echo %{0} re 42
/def -F -p43 -mregexp -t'_x' r43 = /echo %{0} re 43
/def -F -p44 -mregexp -t'a-b' r44 = /echo %{0} re 44
/def -F -p45 -mregexp -t'a,b' r45 = /echo %{0} re 45
/def -F -p46 -mregexp -t'#x' r46 = /echo %{0} re 46
/def -F -p47 -mregexp -t'%%' r47 = /echo %{0} re 47
/def -F -p48 -mregexp -t' ' r48 = /echo %{0} re 48
/def -F -p49 -mregexp -t'^ ' r49 = /echo %{0} re 49
/def -F -p50 -mregexp -t'foo ' r50 = /echo %{0} re 50
/def -F -p51 -mregexp -t'o bar' r51 = /echo %{0} re 51
/def -F -p52 -mglob -t'foo*' g52 = /echo %{0} glob 52
/def -F -p53 -mglob -t'Foo*' g53 = /echo %{0} glob 53
/def -F -p54 -mglob -t'*foo*' g54 = /echo %{0} glob 54
/def -F -p55 -mglob -t'f?o' g55 = /echo %{0} glob 55
/def -F -p56 -mglob -t'{foo|bar}*' g56 = /echo %{0} glob 56
/def -F -p57 -mglob -t'foo' g57 = /echo %{0} glob 57
/def -F -p58 -mglob -t'FOO' g58 = /echo %{0} glob 58
/def -F -p59 -mglob -t'fo[o]*' g59 = /echo %{0} glob 59
/def -F -p60 -mglob -t'café*' g60 = /echo %{0} glob 60
/def -F -p61 -mglob -t'*' g61 = /echo %{0} glob 61
/def -F -p62 -mglob -t'' g62 = /echo %{0} glob 62
/def -F -p63 -mglob -t'{}' g63 = /echo %{0} glob 63
/def -F -p64 -mglob -t'a*c' g64 = /echo %{0} glob 64
/def -F -p65 -mglob -t'A*c' g65 = /echo %{0} glob 65
/def -F -p66 -mglob -t'ab?c' g66 = /echo %{0} glob 66
/def -F -p67 -mglob -t'k*' g67 = /echo %{0} glob 67
/def -F -p68 -mglob -t'K*' g68 = /echo %{0} glob 68
/def -F -p69 -mglob -t'[fF]oo' g69 = /echo %{0} glob 69
/def -F -p70 -mglob -t'foo bar' g70 = /echo %{0} glob 70
/def -F -p71 -mglob -t'foo *' g71 = /echo %{0} glob 71
/def -F -p72 -mglob -t'{foo} bar' g72 = /echo %{0} glob 72
/echo -- subjects
/echo [foo]
/trigger foo
/echo [FOO]
/trigger FOO
/echo [Foo]
/trigger Foo
/echo [a foo]
/trigger a foo
/echo [foobar]
/trigger foobar
/echo [foo bar]
/trigger foo bar
/echo [fo]
/trigger fo
/echo [abbc]
/trigger abbc
/echo [abc]
/trigger abc
/echo [xx]
/trigger xx
/echo [xxx]
/trigger xxx
/echo [café]
/trigger café
/echo [CAFÉ]
/trigger CAFÉ
/echo [Kelvin]
/trigger Kelvin
/echo [Kelvin]
/trigger Kelvin
/echo [kel]
/trigger kel
/echo [ſtop]
/trigger ſtop
/echo [stop]
/trigger stop
/echo [STOP]
/trigger STOP
/echo [ ]
/trigger  
/echo [a-b]
/trigger a-b
/echo [a,b]
/trigger a,b
/echo [#x]
/trigger #x
/echo [100%%]
/trigger 100%%
/echo [oo]
/trigger oo
/echo [o bar]
/trigger o bar
/echo [bar]
/trigger bar
/echo [ab]
/trigger ab
/echo [a]
/trigger a
/echo [b]
/trigger b
/echo [_x]
/trigger _x
/echo [FOO BAR]
/trigger FOO BAR
