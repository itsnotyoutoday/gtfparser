# Differential fuzz findings (real tf vs this engine)

Source: random expression fuzzer (2,500 expressions, 284 divergences), random macro programs
(100, 39 divergences) and a 13-file library corpus (`testdata/cases2/`, 12 of 13 diverge),
all run against engine main v0.0.4. Items are grouped by root cause, most important first.
"tf" = the real binary, "gtf" = this engine. Reproduce with `scripts/fuzz_all.sh`.

## Engine crashes (fix first)
- `/shift -1` panics (index out of range in `shiftArgs`); tf treats it as a no-op.
- `strncmp("abc","abd",-1)` panics; tf returns -1.

## Top findings
1. **`shift()` used as a function in an expression is a no-op.** Breaks lisp.tf `/reverse /mapcar
   /maplist /remove /unique` and stdlib `/nth`. `/def b4 = /test shift()%; /echo %1 %{#}` then
   `/b4 a b c`: tf `b 2`, gtf `a 3`. `/while (shift(), {#}) ...` never iterates.
2. **tf strips trailing whitespace from a command's argument text** (about 40% of macro divergences).
   `/def a3 = /echo trail %{nosuch}` then `/a3`: tf `trail`, gtf `trail ` (shows up in `/wc /alias
   /pop /push` and lisp results).
3. **regmatch captures set in a callee macro are lost on return** (tf keeps them global).
   `/def sp = /split %*%; /echo [%P1][%P2]`, `/sp name = value`: tf `[name][value]`, gtf `[][]`.
4. **Positional-substitution edge forms.** gtf treats unbraced `%1-queue` as a default-value
   expansion (stack-q `/dequeue` returns "queue"); tf does not. In tf `%{2-}`, `%{5-}`, `%{L-}`,
   `%{nosuch-}` swallow the closing `]`; `%{-0}`, `%{#-x}`, `%{-}`, `%{1:}`, `%{ 1}` and bare `%-`
   are errors. gtf accepts all of them. (tf's behaviour looks accidental but stack-q depends on it.)
5. **Implicit return value of a macro called as a function.** If the last command is `/let` or
   `/set`, tf returns `""`, gtf returns 1. A bare `/return` gives 0 in tf, `""` in gtf.
6. **`$(...)`.** `$(/echo x%;/echo y)`: tf runs both commands, gtf splits the outer statement at
   `%;`. Unterminated `$[1 +` echoes nothing in tf, the raw text in gtf.
7. **`strip_attr`, `encode_attr`, `decode_attr` return 0 for everything** (tf: `strip_attr("abc")`
   is `abc`, `decode_attr("a",1)` is `""`).
8. **Operator precedence.** `=~ !~ =/ !/` bind tighter than `< <= > >=` in gtf; in tf they are one
   left-associative level. `5 < 7 =~ 1`: tf 1, gtf 0; `1 < 2 !~ 0`: tf 1, gtf 0.
9. **Builtin argument counts.** tf rejects wrong counts (`strlen("a","b")`, `strcat()`, `sin()`,
   `pow(1)`, `substr("hello")`, `regmatch()` produce no output); gtf evaluates them. tf accepts
   `pow (2,3)` and `strlen ("abc")` (space before the paren); gtf errors. `toupper(s,n)` and
   `tolower(s,n)` take a count in tf (`toupper("abc",1)` is `Abc`).

## Expression engine
- **Numeric model.** Big literals are truncated to 32 bits in tf arithmetic (`+2147483648` is
  -2147483648, `1+4294967296` is 1); gtf stays 64-bit. `1/4294967296` is an error in tf, 0 in gtf.
  `++f`/`--f` on a float truncate to int in tf (1.5 -> 2). `y - --y` is 0 in tf, 1 in gtf. Float
  division by zero yields nan/inf in tf (`0/log10(1)` is `nan`); gtf errors.
- **Float formatting.** tf prints `-2e0` as `-2`, `-tan(0)` as `-0`, NaN/inf as `nan`/`inf`/`-inf`;
  gtf prints `-2.`, `0.`, `NaN.`, `+Inf`, `-Inf`. `2147483648 - d` is `...248.` in tf, `...248.0` in
  gtf. The 15th digit of `tan(x)` can differ.
- **String-to-number.** tf converts the string `"1:30"` via atoi (`tan("1:30")` is tan(1)) and parses
  hex strings (`tan("0x1f")` is tan(31)); gtf uses 5400 and 0.
- **strcmp/strncmp** return the byte difference in tf (`strcmp("A","a")` is -32), -1/0/1 in gtf;
  also affects `vercmp`.
- **substr.** Negative start counts from the end in tf (`substr("hello",-1)` is `o`), negative
  length drops from the end (`substr("hello",2,-1)` is `ll`); gtf ignores both.
- **replace / char.** `replace("","b","banana")` is a no-op in tf, garbage in gtf. `char(n)` is
  n mod 256 in tf (`char(0)` empty, `char(321)` is `A`); gtf emits Unicode or NUL.
- **regmatch.** An invalid regex returns 0 in tf; gtf yields no value.
- **Unicode (UTF-8 build).** tf counts characters, gtf counts bytes: `strlen("日本語")` is 3 in tf, 9
  in gtf; `substr`, `strchr`, `strstr`, `pad` affected. `toupper("ü")` is `ü` in tf (locale), `Ü` in gtf.
- **Misc functions.** `eval("1+1")` gives 0 in tf, 1 in gtf. `getopts("ab","")` gives 0 in tf, 1 in
  gtf. `tfclose(99)` gives -1 in tf, 0 in gtf. `features("256colors")` gives 1 in tf, errors in gtf.
  `filename("~")` expands in tf. `version()` is missing in gtf.

## Macro engine
- `%?` at macro entry shows the stale result of the last command in tf (e.g. the macro number after
  a `/def`); gtf always shows 1.
- `/for i 1.5 3` uses an integer counter in tf, gtf prints 2.5.
- `/if 0%; /then ...` (list form) is false in tf, true in gtf.
- `/ptime` prints `0.0`, `5.0` in tf, `0`, `5` in gtf. Default `isize` is 1 in tf, 0 in gtf;
  `visual` defaults to off in tf, on in gtf.

## Library macros
- `/purgedef foo*` and `/purgedef` do not remove macros. `/untrig /undeft /nogag /nohilite` report
  "No trigger on ..." and leave the definition. `/purgealias` does not purge; `/alias` listing differs.
- The kb functions (`input`, `kbhead`, `kblen`, `kbgoto`, `kb_upcase_word`, ...) were stubs in gtf
  (v0.0.4 now has them; re-check). `kb_nth_word` hung the old engine.

## Environment only (probably not bugs)
`columns()`, `lines()`, `winlines()` give 4000/50/49 in tf's test pty and 80/24 in gtf.

## Real tf crashes and hangs (do NOT replicate)
- `%?` inside a macro called as a function hangs real tf (`/def m1 = /echo [%?]`, `/test E(m1())`).
- `strrep(s, 1.5e300)` and `strrep(1,99999999999999999999)`: tf hangs, gtf runs out of memory
  (gtf should reject absurd sizes instead).
- `pad(100, 4294967296, 1e20)`: tf segfaults, gtf hangs (gtf should bound it).

## Notes
- Real tf emits invalid UTF-8 for some outputs (`tolower("ÀB")`, `char(>127)`); difftest.py crashes
  on them, so Unicode cases live in `testdata/cases2/unicode.tf` and `scripts/fuzz_corpus.py`
  tolerates them.
- `testdata/cases2/stdlib_defs_worlds.tf` uses `/def -t`, `/untrig`, `/nogag`, `/nohilite`, so it
  depends on the trigger/hook rewrite landing.
