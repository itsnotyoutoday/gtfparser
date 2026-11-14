/def go1 = /if (!getopts("abn#s:", "")) /return 0%; /endif%; /echo a=%{opt_a} b=%{opt_b} n=%{opt_n} s=%{opt_s} rest=[%{*}]
/go1 -a -b -n5 -s"can't stop" -- whiz = bang biff
/go1 -ba -n5 whiz
/go1 -as"x y" -bn7 rest here
/go1 no options
/def go2 = /test getopts("x", 0)%; /echo [%{opt_x}] [%{*}]
/go2 -x foo
/go2 foo
