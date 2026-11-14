# Findings

Places where this engine differs from real tf (snippet, expected per real tf, actual), collected while writing the reference. Sections by page group: A functions (strings/math/time/regex/attributes/misc), B files/worlds/terminal/processes, C macros/control/output, D history/system.

## Group A

```
FINDINGS from the Functions-* pages (fork A). Each: snippet / expected per real tf (reference build) / actual in the engine.
Reference outputs were produced by wrapping each line in a macro (so $[] substitution happens) and running real tf.

1. substr with negative positions
   /echo [$[substr("hello",-2)]] [$[substr("hello",-3,2)]]
   real: [lo] [ll]      engine: [hello] [he]      (negative start/length count from the end, per the manual)

2. strstr / strchr / strrchr start-position argument
   /echo [$[strstr("hello","l",3)]] [$[strstr("hello","l",-2)]]    real: [3] [3]     engine: [2] [2]  (start ignored)
   /echo [$[strchr("hello","l",-1)]] [$[strrchr("hello","lh",2)]]  real: [-1] [2]    engine: [2] [3]

3. strcmp / strncmp return value
   /echo [$[strcmp("B","a")]] [$[strncmp("hello","help",4)]]       real: [-31] [-4]  engine: [-1] [-1]
   (real tf returns the character-code difference; only the sign is documented, so this is low priority but scripts that print it differ)

4. tolower/toupper count argument
   /echo [$[tolower("HeLLo",2)]] [$[toupper("hello",2)]]           real: [heLLo] [HEllo]   engine: [hello] [HELLO]

5. ascii()/char() above 127
   /echo [$[ascii(char(233))]] [$[ascii("é")]] [$[strlen(char(233))]]   real (UTF-8 locale): [233] [195] [1]   engine: [195] [195] [2]
   (engine char(233) yields the 2-byte UTF-8 encoding and counts it as 2; real keeps one character. ascii("é") is 195 in both.)
   Also char(10000): real returns "" (forced into the locale's range), engine returns U+2710.

6. mod by zero
   /echo [$[mod(5,0)]]    real: warning "mod: division by zero", the whole expression yields no output   engine: [0] silently

7. non-finite and integer-valued real printing
   /echo [$[sqrt(-1)]] [$[ln(0)]] [$[pow(-8,3)]]    real: [nan] [-inf] [-512]    engine: [NaN.] [-Inf] [-512.]

8. regmatch count when an optional trailing group does not participate
   /echo [$[regmatch("(a)(b)?","a")]]     real: [2]    engine: [3]
   (a middle unset group, "(a)(b)?(c)" on "ac", counts 4 in both)

9. ftime %s (epoch seconds) and other glibc/real-tf extensions
   /echo [$[ftime("%s",1710504000)]]       real: [1710504000]   engine: []

10. whatis(time())
   /echo [$[whatis(time())]]   real: [atime]   engine: [int]      (also real: mktime() is atime, idle() dtime; engine has no atime/dtime types)

11. /nth
   /def t1 = /echo [$(/nth 2 alpha beta gamma)]%; /t1     real: [beta]   engine: [0]    (stdlib /nth: /result {1} > 0 ? shift({1}), {1} : "")

12. /split: P-variables not visible to the calling macro
   /def t4 = /split Hello = world%; /echo [%P1] [%P2]%; /t4     real: [Hello] [world]    engine: [] []
   (regmatch inside the called macro should set the globally visible P variables)

13. /escape with a % in the text
   /def t5 = /echo [$(/escape % 50%)]%; /t5     real: [50]   engine: [50\%]

14. /result value printed
   /result 5            (typed or in a file)   real: prints nothing      engine: prints 5
   /def sq = /result {1}*{1}  then  /echo [$[eval("/sq 7")]]   real: [49]   engine: 49 then [49]
   (a command's /result value must not be echoed; only /return-less top-level macros' output)

15. /expr at the command line
   /expr 3*4    real: no output (stdlib /expr is /result %*)    engine: prints 12   (same cause as 14)

16. mktime and the time-zone variable (stub / env)
   mktime(2024,3,15,12,30)  real: works (returns 1710523800.0 style atime); engine: stub, "not supported by this client"
   /setenv TZ=UTC then ftime("%H:%M %Z",3600)   real: [01:00 UTC]   engine: [19:00 CST] (TZ in tf's environment not applied by ftime)

17. test() function (stub): test("x * 2") with x=3 should be 6 (listed as Not yet supported in Status; noted for completeness)

18. getopts with a separated numeric option argument
   /def foo = /if (!getopts("n#", "")) /return 0%; /endif%; /echo n=%{opt_n}%; /foo -n -3 x
   real: error "expression syntax error: expected operand, found end of body"
   engine: n=0 args=[] with no clear error
```

## Group B

```
FINDINGS (fork B: files, streams, worlds, terminal, processes)
Real tf = reference build (TFREF_DIR tfwide), run through scripts/difftest.py harness with the echo macro captured.
Engine = /tmp/gtfrun at commit "docs: cmd/gtfdoc generates the builtin status table".

1. tfopen() modes "q" and "p" not supported
   snippet:  /test q := tfopen("jobs","q")   /echo $[tfwrite(q,"a b")] $[tfread(q,v)] %v
             /test p := tfopen("echo hi; echo there","p")   /echo $[tfread(p,v)] %v
   real:     q handle is a positive int; tfwrite 1; tfread returns 3 and v="a b" (FIFO; then -1 when empty).
             p: tfread returns 2, v="hi"; then 5 "there"; then -1.
   engine:   tfopen returns -1 for both (falls through to a plain file open of the name). tfopen() with no args also -1
             (real: unnamed queue). /quote, $() and %| in real tf use queues, so this also limits redirection.

2. tfclose() return values
   snippet:  /test h := tfopen("/tmp/x","w")  /echo $[tfclose(h)]   /echo $[tfclose("e")]  /echo $[tfclose(12345)]
   real:     0 / -1 (tferr cannot be closed) / -1 (bad handle)
   engine:   1 / 1 / 1
   Same for tfflush(12345): real -1, engine 1.

3. tfwrite() on an unusable handle
   snippet:  /test h := tfopen("/tmp/x","r")  /echo $[tfwrite(h,"x")]  /test tfclose(h)  /echo $[tfwrite(h,"x")]  /echo $[tfwrite(99,"x")]
   real:     -1 for all three (plus a "bad handle" message for the closed one)
   engine:   0 for all three
   snippet:  /echo $[tfwrite("e","to tferr")]
   real:     1 (text shown on screen as an error-stream line)   engine: 0, nothing written.

4. tfreadable()
   snippet:  /test w := tfopen("/tmp/x","w")  /echo $[tfreadable(w)]  /test tfclose(w)  /echo $[tfreadable(w)]  /echo $[tfreadable("o")]
   real:     1 1 1   (tfreadable means "tfread would not block": 1 for files, queues, closed/bad handles; 0 only for the
             keyboard "i" and for a pipe with nothing yet)
   engine:   0 0 0 (write-only, closed, "o")

5. Builtin commands callable as functions
   snippet:  /echo $[load("/tmp/a.tf")]   /echo $[ps()] $[kill(99)]   /echo $[repeat("-S 2 /echo hey")]
   real:     work (manual: "a builtin command called as a function can have 0 or 1 arguments; the argument is a command line");
             load() returns 1, ps() 1 (table printed), repeat(...) runs it and returns 1.
   engine:   "load: no such function", "ps: no such function", etc. (and kill/quote/sh/repeat/liststreams/listworlds...).
   Note the library's /sys etc. do not depend on this.

6. /ps -r and /ps -q ignore their filters
   snippet:  /repeat -3600 i /echo a   /quote -3600 :x `/echo b   /echo $(/ps -r -s)   /echo $(/ps -q -s)
   real:     1  /  2
   engine:   1 2  /  1 2

7. /sh -q
   snippet:  /sh -q echo quiet > /tmp/q.txt
   real:     runs the command quietly (file contains "quiet")
   engine:   passes "-q" to the shell: "sh: -q: invalid option" and prints sh's usage.

8. /ps with no options
   real prints a table (PID NEXT T D WORLD PTIME COUNT COMMAND); engine prints "% 1 r 0.009975916 /echo timed"
   (one terse line per process). The documentation shows only /ps -s, which agrees.

9. Asynchronous processes in gtfrun -i
   real tf runs /repeat -0.01 2 /echo timed (shows 'timed' twice) ; under gtfrun -i the process is created (listed
   by /ps) but never fires, because gtfrun has no event loop. Not an engine bug; why async examples are tf-noverify.

10. /lcd error text: real "LCD: <dir>: No such file or directory", engine "% /lcd: chdir <dir>: no such file or directory".
    /load error: real "LOAD: <file>: No such file or directory", engine "% /load: <file>: no such file". Wording only.

11. Quirk of real tf (not an engine bug, noted because the docs would invite the error): status_width("nosuchfield")
    silently aborts the whole running script in the reference build (null dereference guard) instead of returning 0.
```

## Group C

```
C1. /exit inside a loaded file
  snippet (file loaded with /load or -f):
      /def ex = /echo before%; /exit%; /echo after
      /ex
      /echo still
  expected (real tf): "before" only; /exit aborts the macro AND the enclosing /load.
  actual (engine): "before" then "still": the rest of the file keeps loading.

C2. %max_iter
  snippet:
      /set max_iter=2
      /def spin = /let q=0%; /while (1) /test ++q%; /done%; /echo stopped at %q
      /spin
  expected (real tf, reference build): max_iter is documented but unused; the loop runs until
      "spin: instruction count exceeded %max_instr (1000000)" aborts the macro; nothing echoed.
  actual (engine): honours max_iter, echoes "stopped at 2".

C3. /def -1 (one-shot, same as -n1)
  snippet:  /def -1 -t'once' o1 = /echo only once
            /trigger once
            /trigger once
  expected (real tf): "only once" printed once, then the macro is gone.
  actual (engine): "/def: invalid option '1'"; macro not defined.

C4. /def -w<world> with an unknown world
  snippet:  /def -wtf1 -t'wmatch' ww = /echo ww
            /trigger wmatch
  expected: "% DEF -w: No world tf1" and no macro.   actual: macro defined silently and fires.

C5. /trigger options
  snippet:  /def -hCONNECT hc = /echo connect hook
            /trigger -hCONNECT
            /def -h'DISCONNECT' hd = /echo dc
            /trigger -hDISCONNECT
  expected: hook macros run ("connect hook", "dc"); -n lists matches, -l lists them in full, -w/-g restrict scope,
            -h with a bad event name gives "invalid hook event".
  actual:   /trigger ignores all options and just feeds the rest of the line to ProcessLine
            (hooks never fire, -n/-l are not honoured, return value is always 1).
            Use Interp.Hook(event,args) from Go to fire hooks.

C6. /listvar -x / /export
  snippet:  /setenv EVAR=abc
            /listvar -x EVAR
  expected: "/setenv EVAR=abc" (exported variables are listed as /setenv).   actual: "/set EVAR=abc".

C7. /listvar lists a variable `version` the reference build does not have
  snippet:  /listvar -s v*     expected: virtscreen visual   actual: version virtscreen visual (extra "version").

C8. /undef return value
  snippet:  /def a=x%; /def b=y%; /undef a b nosuch%; /echo %?   (inside a macro/eval)
  expected: 2 (number of macros removed; "% UNDEF: Macro "nosuch" was not defined." warning).
  actual:   1 and no warning.

C9. /nth (stdlib: /def -i nth = /result {1} > 0 ? shift({1}), {1} : "")
  snippet:  /nth 2 x y z
  expected: y     actual: "/return: expression: expected ':' in conditional" and nothing echoed
  (comma operator inside the ? : branch is not parsed; related: function shift() may be missing).

C10. /split (stdlib: /def -i split = /@test regmatch("^([^=]*[^ =])? *=? *(.*)", {*}))
  snippet:  /split key = value here%; /echo [%P1] [%P2]
  expected: [key] [value here]     actual: [] []

C11. /untrig with a glob metacharacter pattern
  snippet:  /def -t'ant*' t1 = /echo t1
            /untrig ant*
            /trigger ant
  expected: t1 removed, silent, nothing echoed.   actual: "% No trigger on ant*." and t1 still fires.
  (/untrig uses `/purge -msimple -t"$(/escape " ant*)"`; the escaped pattern is not matched literally.)

C12. /def return value
  snippet:  /def dd1 = x%; /echo %?
  expected: the new macro's number (e.g. 762); actual: 1.

C13. builtin command called as a function
  snippet:  /test def("-t'zap' zz = /echo zap")
            /trigger zap
  expected: "zap".  actual: "/test: def: no such function".

C14. /gag and /hilite priority
  snippet:  /gag *spam*   then   /list
  expected: /def -p0 -ag -mglob -t'*spam*'  (priority is %gpri / %hpri, default 0).  actual: -p1.
  Also: real tf's /listhilite and /listpartial fail in the reference build ("LIST -P: "-P" requires
  "-mregexp -t<pattern>"") while the engine accepts them.

C15. fall-through to a LOWER priority trigger
  snippet:  /def -t'foo' -F -p7 fa = /echo fa
            /def -t'foo' -p2 fb = /echo fb
            /trigger foo
  expected (real tf): fa then fb (-F lets lower priorities run).
  actual (engine): only fa. (Equal priorities, -p1 both, work: fa fb.)
```

## Group D

```
F-D1  /sys return value (library macro; docs/commands/sys.html)
  snippet:   /sys exit 3
             /test E(strcat("[", {?}, "]"))
  expected (real tf): [3]   (exit status of the shell; /sys echo hi -> [0])
  actual (engine):    [1]   in both cases

F-D2  builtin commands called as functions (topics/functions.html: "A builtin command called as a function can have 0 or 1 arguments")
  snippet:   /test set("qq=1")
             /test E(qq)
             /test E(version())
             /test E(limit())      <- used by the stock tfstatus.tf
  expected (real tf): 1 / 1 / 1 / 0 (command run, result returned)
  actual (engine):    "% /test: set: no such function" etc.; only names registered as functions work

F-D3  /version text vs /ver and /changes
  snippet:   /test echo(ver())
  expected (real tf): the version, e.g. "development" (stdlib's /ver matches
             'version (.*). % Copyright' against the multi-line $(/version))
  actual (engine):    empty string; /version prints a single "% TinyFugue (gtf) version 5.0 beta 8"
             line with no copyright line, so the regexp never matches (and /changes, which defaults
             to $(/ver), cannot work).

F-D4  /sys/quote output prefix (not confirmed as a bug, noted for follow-up)
  snippet:   /sys echo hi        (engine run with the harness' plain echo)
  actual (engine): prints "!hi"; real tf's visible output could not be captured in this harness
  (stdout of the shell goes through /echo); worth a direct pty comparison.

F-D5  /visual on without a capable terminal
  snippet:   /set visual=off
             /visual on
             /test E(visual)
  expected (real tf, non-tty/dumb terminal): off  (refuses visual mode)
  actual (engine):    on  (stores the value; the host decides what to draw). Intentional, documented in Commands-History-and-Display.md.
```
