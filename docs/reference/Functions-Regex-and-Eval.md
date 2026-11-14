# Functions: Regex and Eval

These functions connect tf's expression language to the rest of the engine:
`regmatch` runs a regular expression and keeps its captures, `eval` and `test`
run text as a macro body or an expression, `getopts` gives your own macros
real command-line options, and `substitute` rewrites the line a trigger is
processing.

Regular expressions are Perl-compatible (PCRE2 syntax: `\d`, `(?i)`,
non-greedy `+?`, named groups, lookahead). Remember that inside a tf string a
backslash must itself be escaped, so the pattern `\d+` is written `"\\d+"`.

## regmatch(pattern, string)

**What it is for:** testing whether a line matches a pattern and picking pieces out of it: parsing prompts, scoring lines, reading numbers out of game messages.

**Syntax:** `regmatch(s1, s2)`: regexp `s1`, text `s2`. Returns an integer.

**Returns:** 0 if the text does not match; otherwise a positive count of the captured substrings including the whole match. After a successful match these are available as `%{P0}` (all of the matched text), `%{P1}`, `%{P2}`... (the parenthesised groups, in order), `%{PL}` (the text left of the match) and `%{PR}` (the text right of it). Variable names `P1`... work in expressions as `{P1}`.

```tf
/echo [$[regmatch("(\\w+) tells you: (.*)", "Bob tells you: meet me at the inn")]]
/echo who=%{P1} what=%{P2}
/echo [$[regmatch("(\\d+)/(\\d+) hp", "You have 30/120 hp.")]] $[{P1} * 100 / {P2}]%
/echo [$[regmatch("zzz", "abc")]]
/echo [$[regmatch("(?i)^HELLO", "hello there")]] [$[regmatch("HELLO", "hello there")]]
```
```output
[3] 
who=Bob what=meet me at the inn
[3] 25%
[0]
[1] [0]
```

**Gotchas:** the `%{P1}` variables are filled in only when the function runs, so read them on a later line or later in the same macro, not in the same `/echo` argument that calls `regmatch`. Do not rely on the `P` variables after a match that returned 0. Patterns match anywhere in the string unless anchored with `^` and `$`; matching is case sensitive unless `(?i)` is used (the `-mregexp` of macros is separate and goes through the same engine). An invalid pattern is reported as an error and the call returns 0. Real tf counts only up to the last group that took part in the match; the engine currently counts all groups when an optional trailing group did not match (FINDINGS.md).

**Status:** see [Status](Status.md).

## eval(text [, level])

**What it is for:** running text that you built at run time as if it were a macro body: building command names, running a command stored in a variable, or delayed substitution.

**Syntax:** `eval(s)` or `eval(s, level)`; also available as the command `/eval [-slevel] text`.

**Returns:** the return value of the last command executed in `text` (a command that succeeds gives 1; `eval("/sq 7")` with `/def sq = /result {1}*{1}` gives 49). The text goes through full substitution first (`%name`, `$[expr]`, `%;`), runs in a new variable scope and inherits the caller's positional parameters. The optional `level` selects the substitution mode as the `sub` variable does (`0`/`off`: none, `on`: variables only, `full`).

```tf
/set name=Bob
/echo [$[eval("/echo hello %name")]]
/def say_with = /eval /echo %%name was told: %{*}
/say_with come here
/def lit = /eval -s0 /echo %%name stays literal
/lit
```
```output
hello Bob
[1]
Bob was told: come here
%name stays literal
```

**Gotchas:** the doubled `%%name` in a macro body survives the first substitution (at `/def` run time) and is expanded by `eval`. Never `eval` text that came from another player without `-s0` or sanitising: a hostile line could carry commands, as with any trigger that runs `%*`. When `text` is plain words with no leading `/`, it is sent to the world just as if typed, so an `eval` of ordinary text depends on a connection. A `/return` inside the evaluated text ends the evaluated code only. Typed at the command line the value of a `/result` is not printed in real tf (the engine currently prints it; see FINDINGS.md), so print results yourself with `/echo`.

**Status:** see [Status](Status.md).

## test(expression)

**What it is for:** evaluating an expression held in a string, for calculators, user-supplied conditions and configuration values.

**Syntax:** `test(s)`; the string is parsed as an expression and its value is returned (compare the command `/test expr`, which runs an expression and sets `%?`).

**Returns:** the value of the expression, of whatever type.

```tf-noverify
/set x=3
/echo $[test("x * 2")]
/echo $[test("x > 1 & x < 9")]
/echo $[test("strlen('abc') == 3")]
```
```output
6
1
1
```

**Gotchas:** an unknown name evaluates to an empty value rather than an error. The command `/test` is the usual way to call a function purely for its side effect (`/test regmatch("foo(.*)", "foobar")` then `%{P1}` is `bar`).

**Status:** see [Status](Status.md).

## getopts(options [, init])

**What it is for:** making your own macros accept options like a built-in command (`/mycmd -f -n5 -s"some text" rest`): the heart of any polished macro.

**Syntax:** `getopts(s1)` or `getopts(s1, s2)`. `s1` lists the accepted option letters; a letter followed by `:` takes a string argument, `#` an integer expression, `@` a time. `s2` initialises every `opt_X` variable before parsing.

**Returns:** non-zero if the options were valid, or 0 after printing an error (an unknown option also prints the list of valid ones). For each option found it sets a local variable `opt_X` (the argument, or 1 for a plain flag) and shifts the options off the argument list, so `%{*}` afterwards holds only the remaining words.

```tf
/def hunt = /if (!getopts("an#s:", "")) /return 0%; /endif%; /echo a=[%{opt_a}] n=[%{opt_n}] s=[%{opt_s}] rest=[%{*}]
/hunt -a -n5 -s"giant rat" cellar door
/hunt -- -a
/hunt plain words
```
```output
a=[1] n=[5] s=[giant rat] rest=[cellar door]
a=[] n=[] s=[] rest=[-a]
a=[] n=[] s=[] rest=[plain words]
```

**Gotchas:** `--` ends option parsing, so arguments that start with a dash can follow it. Options may be bundled (`-an5`) and arguments attached (`-sfoo`) or quoted. Without the `init` argument unset variables are left alone, so set defaults first with `/let opt_x=dflt` and a call such as `getopts("x:")` then keeps them if the option is absent. Always `/return 0` when `getopts` fails, otherwise the macro carries on with junk.

**Status:** see [Status](Status.md).

## substitute(text [, attrs [, inline]])

**What it is for:** inside a trigger, replacing the line that fired the trigger with different text, for example to shorten spam, translate codes or strip a prefix before lower-priority triggers and the screen see it.

**Syntax:** `substitute(s)`, `substitute(s, attrs)`, `substitute(s, attrs, inline)`; the command form is `/substitute [-aattrs] [-p] text`.

**Returns:** 1 on success, 0 if called outside a trigger ("not called from trigger").

```tf-noverify
/def -mglob -ah -t'##page> *' strip_page = /substitute %-1
```
```output
```

`-ah` hides the original; the substituted text is what the player sees and what later triggers match.

**Gotchas:** `attrs` are added to the original line's attributes unless they include `x`. With `inline` set to 1 (`-p` for the command) `@{attr}` codes in `text` are interpreted. Only the *current* line is affected, and only while the trigger runs.

**Status:** see [Status](Status.md).

## Library

### /expr expression (Library)

**What it is for:** returning an expression's value from the command line so `$(...)` can use it.

**Syntax:** `/expr expression` (defined as `/result %*`).

```tf
/set gold=50
/echo double is $(/expr gold * 2)
```
```output
double is 100
```

**Gotchas:** it returns rather than prints; use `/echo` or `$(...)` to show it.

**Status:** library macro from `stdlib.tf`; see [Status](Status.md).

### /not command (Library)

**What it is for:** negating the success code of another command, in the form `/not command`; `/!command` is the cleaner built-in syntax.

**Syntax:** `/not text` evaluates `text` as a macro body (like `/eval`) and returns the logical negation of its result.

```tf
/def tell_not = /not /false%; /echo after /not /false: %?
/tell_not
```
```output
after /not /false: 1
```

**Gotchas:** `/not` evaluates its arguments (substitutions included), which can surprise: prefer `/!cmd`.

**Status:** library macro from `stdlib.tf`; see [Status](Status.md).

### /ismacro name and /isvar name (Library)

**What they are for:** asking whether a macro, or a variable, exists, before relying on it in a script.

**Syntax:** `/ismacro name`, `/isvar name`; they set `%?` (`/ismacro` returns the macro number, so any non-zero value means yes; `/isvar` returns 1) and print nothing.

```tf
/def hello = /echo hi
/set gold=50
/def probe = /ismacro nosuch%; /echo nosuch macro: %?%; /isvar gold%; /echo gold var: %?%; /isvar nogold%; /echo nogold var: %?
/probe
```
```output
nosuch macro: 0
gold var: 1
nogold var: 0
```

**Gotchas:** the macro number for an existing macro changes from session to session; compare against 0 only.

**Status:** library macros from `stdlib.tf`; see [Status](Status.md).
