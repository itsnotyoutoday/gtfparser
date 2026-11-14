# Findings

Places where this engine still differs from real tf at v0.0.5 (snippet, expected per real tf, actual), collected while writing the reference. Entries that no longer reproduce were dropped. Groups: A functions (strings/math/time/regex/attributes/misc), B streams/worlds/terminal/files/processes, C macros/control/output, D history/system.

## Group A

```
FINDINGS from the Functions-* pages, re-tested on v0.0.5 (engine main). Each: snippet / expected per real tf (reference build) / actual in the engine. Items 14, 15 and 18 of the first pass no longer reproduce and were dropped.

1. substr with negative positions  [still open at v0.0.5]
   /echo [$[substr("hello",-2)]] [$[substr("hello",-3,2)]]
   real: [lo] [ll]      engine: [hello] [he]      (negative start/length count from the end, per the manual)

2. strstr / strchr / strrchr start-position argument  [still open at v0.0.5]
   /echo [$[strstr("hello","l",3)]] [$[strstr("hello","l",-2)]]    real: [3] [3]     engine: [2] [2]  (start ignored)
   /echo [$[strchr("hello","l",-1)]] [$[strrchr("hello","lh",2)]]  real: [-1] [2]    engine: [2] [3]

3. strcmp / strncmp return value  [still open at v0.0.5]
   /echo [$[strcmp("B","a")]] [$[strncmp("hello","help",4)]]       real: [-31] [-4]  engine: [-1] [-1]
   (real tf returns the character-code difference; only the sign is documented, so this is low priority but scripts that print it differ)

4. tolower/toupper count argument  [still open at v0.0.5]
   /echo [$[tolower("HeLLo",2)]] [$[toupper("hello",2)]]           real: [heLLo] [HEllo]   engine: [hello] [HELLO]

5. ascii()/char() above 127  [still open at v0.0.5]
   /echo [$[ascii(char(233))]] [$[ascii("é")]] [$[strlen(char(233))]]   real (UTF-8 locale): [233] [195] [1]   engine: [195] [195] [2]
   (engine char(233) yields the 2-byte UTF-8 encoding and counts it as 2; real keeps one character. ascii("é") is 195 in both.)
   Also char(10000): real returns "" (forced into the locale's range), engine returns U+2710.

6. mod by zero  [still open at v0.0.5]
   /echo [$[mod(5,0)]]    real: warning "mod: division by zero", the whole expression yields no output   engine: [0] silently

7. non-finite and integer-valued real printing  [still open at v0.0.5]
   /echo [$[sqrt(-1)]] [$[ln(0)]] [$[pow(-8,3)]]    real: [nan] [-inf] [-512]    engine: [NaN.] [-Inf] [-512.]

8. regmatch count when an optional trailing group does not participate  [still open at v0.0.5]
   /echo [$[regmatch("(a)(b)?","a")]]     real: [2]    engine: [3]
   (a middle unset group, "(a)(b)?(c)" on "ac", counts 4 in both)

9. ftime %s (epoch seconds) and other glibc/real-tf extensions  [still open at v0.0.5]
   /echo [$[ftime("%s",1710504000)]]       real: [1710504000]   engine: []

10. whatis(time())  [still open at v0.0.5]
   /echo [$[whatis(time())]]   real: [atime]   engine: [int]      (also real: mktime() is atime, idle() dtime; engine has no atime/dtime types)

11. /nth  [still open at v0.0.5]
   /def t1 = /echo [$(/nth 2 alpha beta gamma)]%; /t1     real: [beta]   engine: [0]    (stdlib /nth: /result {1} > 0 ? shift({1}), {1} : "")

12. /split: P-variables not visible to the calling macro  [still open at v0.0.5]
   /def t4 = /split Hello = world%; /echo [%P1] [%P2]%; /t4     real: [Hello] [world]    engine: [] []
   (regmatch inside the called macro should set the globally visible P variables)

13. /escape with a % in the text  [still open at v0.0.5]
   /def t5 = /echo [$(/escape % 50%)]%; /t5     real: [50]   engine: [50\%]

16. mktime and the time-zone variable (stub / env)  [still open at v0.0.5]
   mktime(2024,3,15,12,30)  real: works (returns 1710523800.0 style atime); engine: stub, "not supported by this client"
   /setenv TZ=UTC then ftime("%H:%M %Z",3600)   real: [01:00 UTC]   engine: [19:00 CST] (TZ in tf's environment not applied by ftime)

17. test() function (stub): test("x * 2") with x=3 should be 6 (listed as Not yet supported in Status; noted for completeness)  [still open at v0.0.5]


19. Script values carry no attributes (documented engine limitation, docs/engine/triggers-hooks.md)  [still open at v0.0.5]
   /eval /echo [$[encode_attr(decode_attr("@{BCred}Danger@{n} ahead"))]]
   real: [@{BCred}Danger@{n} ahead]     engine: [Danger ahead]
   Same for encode_ansi(decode_attr(...)) (no escape codes), encode_attr(decode_ansi(...)), decode_attr("x","Cgreen"), and strcmpattr(decode_attr("@{B}abc"),"abc") (real: non-zero, engine: 0).
```

## Group Bfunc

```
FINDINGS (group B: files/streams, worlds, terminal). Re-tested on engine v0.0.5+ (docs-ref rebased onto main).
Real tf = reference build, run through scripts/difftest.py.

1. [still open at v0.0.5] tfopen() modes "q" and "p", and tfopen() with no arguments
   snippet:  /test q := tfopen("jobs","q")   /echo $[tfwrite(q,"a b")] $[tfread(q,v)] %v
             /test p := tfopen("echo hi; echo there","p")   /echo $[tfread(p,v)] %v
   real:     q is a positive handle; tfwrite 1; tfread 3, v="a b" (FIFO, then -1 when empty).
             p: tfread 2 with v="hi", then 5 with "there", then -1. tfopen() opens an unnamed queue.
   actual:   tfopen returns -1 for "q" and "p" (it falls through to a plain file open); tfopen() returns -1.

2. [still open at v0.0.5] tfclose()/tfflush() return values
   snippet:  /test h := tfopen("/tmp/x","w")  /echo $[tfclose(h)] $[tfclose("e")] $[tfclose(12345)] $[tfflush(12345)]
   real:     0 -1 -1 -1
   actual:   1 1 0 1

3. [still open at v0.0.5] tfwrite() on an unusable handle
   snippet:  /test h := tfopen("/tmp/x","r")  /echo r=$[tfwrite(h,"x")]  /test tfclose(h)  /echo r=$[tfwrite(h,"x")] r=$[tfwrite(99,"x")]
   real:     -1 for all three (plus a "bad handle" message for the closed one)   actual: 0 for all three
   snippet:  /echo $[tfwrite("e","to tferr")]    real: 1 (text appears as an error line)   actual: 0, nothing written

4. [still open at v0.0.5] tfreadable()
   snippet:  /test w := tfopen("/tmp/x","w")  /echo $[tfreadable(w)]  /test tfclose(w)  /echo $[tfreadable(w)] $[tfreadable("o")]
   real:     1 1 1 (it means "tfread would not block": 1 for files, queues, closed and bad handles; 0 only for the keyboard "i" and an empty pipe)
   actual:   0 0 0

5. [still open at v0.0.5] builtin commands called as functions
   snippet:  /echo $[load("/tmp/a.tf")]   /echo $[ps()]   /echo $[unworld("Hub2")]   /echo $[repeat("-S 2 /echo hey")]
   real:     work (a builtin command called as a function takes 0 or 1 argument, a command line)
   actual:   "% load: no such function" (same for ps, unworld, kill, repeat, listworlds, ...)

6. [still open at v0.0.5] fwrite(), read(), nread(), nmail(), isatty() are still stubs ("not supported by this client"), see Status.md.

7. [new, still open at v0.0.5] /else after a nested one-line /if in a false /if branch
   snippet:  /def t2 = /if (0) /if (1) /echo a%; /endif%; /else /echo closed%; /endif
             /t2
   real:     closed
   actual:   "% /else without matching /if or /while", "% /endif without matching /if or /while", no output
   Consequence: the stock library macro /world (and so /world NAME) fails with those errors whenever the world is not open
   (it nests `/if (opt_x) ...%; /endif` inside the `/else` of `/if (is_open(...))`); the doc example for /world is therefore tf-noverify.

8. [new, still open at v0.0.5] difftest compares only echoed output, so message streams are not covered here: world/socket
   messages ("% Not connected to any sockets.", "% World renamed.") were checked by eye against the real binary only.

Dropped (no longer reproduce / not engine bugs): world database commands and functions (/addworld /unworld /renameworld /listworlds
/saveworld, addworld(), world_info()), kb* functions, status_fields(), status_width(), winlines(), moresize(), morepaused(),
morescroll(), nactive(), idle(), sidle(), is_open(), is_connected(), fg_world(), nlog() are implemented and match real tf in the
no-socket case (difftest ok). Not engine bugs, kept as notes in the pages: real tf aborts the script on
status_width("nosuchfield") (the engine warns and returns 0); asynchronous /repeat and /quote never fire under gtfrun -i (no event loop).
```

## Group Bfiles

```
FILES / STREAMS / PROCESSES (re-tested on engine main after v0.0.5; real tf = reference build)

1. tfopen() modes "q" and "p" -- still open at v0.0.5
   snippet:  /test q := tfopen("jobs","q")   /test tfwrite(q,"a b")   /test tfread(q,v)
             /test p := tfopen("echo hi; echo there","p")   /test tfread(p,v)
             /test tfopen()
   real:     q handle is a positive int; tfwrite 1; tfread returns 3 and v="a b"; p: tfread returns 2, v="hi", then 5 "there"; tfopen() 4 (unnamed queue)
   engine:   tfopen returns -1 for all three; tfwrite 0, tfread -1.

2. tfclose()/tfflush() return values -- still open at v0.0.5
   snippet:  /test h := tfopen("/tmp/x","w")  /test tfclose(h)  /test tfclose("e")  /test tfclose(12345)  /test tfflush(12345)
   real:     0 / -1 / -1 / -1
   engine:   1 / 1 / 0 / 1

3. tfwrite() on an unusable handle -- still open at v0.0.5
   snippet:  h opened "r": tfwrite(h,"x"); after tfclose(h): tfwrite(h,"x"); tfwrite(99,"x"); tfwrite("e","to tferr")
   real:     -1 -1 -1 / 1 (the text appears as an error line)
   engine:   0 0 0 / 0

4. tfreadable() -- still open at v0.0.5
   snippet:  w := tfopen("/tmp/x","w"); tfreadable(w); tfreadable("o"); after tfclose(w): tfreadable(w)
   real:     1 1 1 (it means "tfread would not block")
   engine:   0 0 0

5. Builtin commands called as functions -- still open at v0.0.5
   snippet:  /test ps()   /test repeat("-S 2 /echo hey")   /test load("/tmp/a.tf")   /test set("qq=1")   /test features("float")
   real:     the command line runs and the command's result is returned (repeat prints "hey" twice)
   engine:   "ps: no such function" etc. (features() returns 0 with a warning, which also makes the stock library print
             "% features: no such function" and "% /endif without matching /if or /while" at load time)

6. /ps -r and /ps -q ignore their filters -- still open at v0.0.5
   snippet:  /repeat -3600 i /echo a   /quote -3600 :x `/echo b   /echo $(/ps -r -s)   /echo $(/ps -q -s)
   real:     1 / 2        engine: 1 2 / 1 2

7. /sh -q -- still open at v0.0.5
   snippet:  /sh -q echo quiet > /tmp/q.txt
   real:     runs the command quietly      engine: passes "-q" to the shell ("sh: -q: invalid option")

8. /ps without -s -- still open at v0.0.5
   real prints a table headed "  PID     NEXT T D WORLD       PTIME COUNT COMMAND" (the header even when no process matches), e.g.
   "    3  0:04:59 r             0:05:00     i /send look"; engine prints one line per process: "% 3 r 299.99998 /send look".

9. /lcd and /load error wording -- still open at v0.0.5
   real "LCD: /x: No such file or directory", "LOAD: /f.tf: No such file or directory";
   engine "% /lcd: chdir /x: no such file or directory", "% /load: /f.tf: no such file".

10. NEW: /qdef (quoter.tf) substitutes positional parameters in the quoted definition
    snippet:  /require quoter.tf   /def greet = /echo Hello, %1!   /def -i send = /echo SEND %*   /qdef greet
    real:     SEND - :| /def greet = /echo Hello, %1!
    engine:   SEND - :| /def greet = /echo Hello, greet!

(Not an engine bug: asynchronous /repeat and /quote never fire under gtfrun -i, which has no event loop; that is why such
examples are tf-noverify. And real tf's status_width("nosuchfield") aborts the running script.)
```

## Group C

```
All entries below were re-tested on the rebased engine (v0.0.5+); C3, C5, C6, C7, C8, C9, C11, C12, C14, C15
from the earlier list are fixed and dropped (C7: no extra `version`; C14: /gag and /hilite use gpri/hpri;
C14's "real /listhilite fails with -P" now matches in both).

C1. /exit inside a loaded file                                          still open at v0.0.5
  snippet (file loaded with /load or -f):
      /def ex = /echo before%; /exit%; /echo after
      /ex
      /echo still
  expected (real tf): "before" only; /exit aborts the macro AND the enclosing /load.
  actual (engine): "before" then "still".

C2. %max_iter                                                           still open at v0.0.5
  snippet:
      /set max_iter=2
      /def spin = /let q=0%; /while (1) /test ++q%; /done%; /echo stopped at %q
      /spin
  expected (real tf, reference build): max_iter is unused; the loop runs until
      "spin: instruction count exceeded %max_instr (1000000)" aborts the macro; nothing echoed.
  actual (engine): honours max_iter, echoes "stopped at 2".

C4. /def -w<world> with an unknown world                                still open at v0.0.5
  snippet:  /def -wtf1 -t'wmatch' ww = /echo ww
            /trigger wmatch
  expected: "% DEF -w: No world tf1" and no macro.
  actual:   macro defined and fires (the check needs a WorldInfoHost; gtfrun has none).

C10. /split (stdlib: /def -i split = /@test regmatch("^([^=]*[^ =])? *=? *(.*)", {*}))   still open at v0.0.5
  snippet:  /split key = value here%; /echo [%P1] [%P2]   (also via /eval)
  expected: [key] [value here]     actual: [] []
  (%P1/%P2 set by a regmatch() inside the library macro are not visible to the caller.)

C13. builtin command called as a function                               still open at v0.0.5
  snippet:  /test def("-t'zap' zz = /echo zap")
            /trigger zap
  expected: "zap".  actual: "/test: def: no such function".
```

## Group D

```
HISTORY / DISPLAY / SYSTEM (re-tested on engine main after v0.0.5; real tf = reference build)

Fixed since the first pass (dropped): /recall, /recordline, /histsize, /status_*, /clock, /status_save and /watchdog/watchname
now behave as real tf on every example in the reference (checked with scripts/difftest.py).

F-D1  /sys return value -- still open at v0.0.5
  snippet:   /sys exit 3
             /echo [%?]
  real tf:   [3]  (the shell's exit status)       engine: [1]

F-D2  builtin commands called as functions -- still open at v0.0.5
  snippet:   /test set("qq=1")  /echo [%qq]   /test version()   /echo [$[features("float")]]
  real tf:   [1], and features("float") is 1       engine: "set: no such function"; qq empty; features() 0 with a warning
  (limit() itself now returns the command's value, 0 here, and matches real tf.)

F-D3  /version text vs the library's /ver and /changes -- still open at v0.0.5
  snippet:   /echo [$(/ver)]
  real tf:   [development]   (stdlib /ver matches 'version (.*). % Copyright' against the multi-line /version output)
  engine:    []              (/version prints one "% TinyFugue (gtf) version 5.0 beta 8" line and no copyright line;
             /changes, which defaults to $(/ver), cannot work either)

F-D4  /sys output prefix -- unconfirmed, still open at v0.0.5
  snippet:   /sys echo hi          engine prints "!hi" through its harness; real tf's visible text could not be captured
  without a pty comparison.

F-D5  /visual on without a capable terminal -- intentional difference, still open at v0.0.5
  snippet:   /set visual=off  /visual on  /echo %visual
  real tf (dumb terminal): off     engine: on (the host decides what to draw)
```
