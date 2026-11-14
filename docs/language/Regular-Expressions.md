# Regular expressions

Regexp patterns (`/def -mregexp`, `regmatch()`) use a PCRE2-compatible engine
(the `pcre` package in this repository), the same dialect real tf uses. This page
lists the defaults that differ from what you may expect from Perl or JavaScript,
how captures reach your macro, and what the engine does not support.

## Using a regexp

Two ways:

* A **trigger or hook** defined with `-mregexp` (or `-P`).
* The **`regmatch(pattern, text)`** function, usable anywhere in an expression.
  It returns nonzero when the pattern matches somewhere in `text`.

Regexps are **not anchored**: the match may occur anywhere. Add `^` and `$`
yourself to match whole lines.

```tf
/def anywhere = /echo $[regmatch('orc', 'a big orc!')] $[regmatch('^orc$', 'a big orc!')] $[regmatch('^orc$', 'orc')]
/anywhere
```
```output
1 0 1
```

## Captures: `%P0`, `%P1`, ...

After a successful match, the text of each parenthesised group is available as
`%P1`, `%P2`, ... in order of the opening parenthesis. `%P0` is the whole matched
text, `%PL` the text to its left, and `%PR` the text to its right. A group
that did not take part in the match is empty, and a number with no such group
is also empty. In an expression write `{P1}`.

In a **regexp trigger** the captures are ready when the macro body runs:

```tf
/def -mregexp -t'^(\w+) hits you for (\d+)' dmg = /echo %P1 did %P2 (%{P0}) [%PL][%PR]
/trigger Orc hits you for 12
/trigger Orc hits you for 12 points
```
```output
Orc did 12 (Orc hits you for 12) [][]
Orc did 12 (Orc hits you for 12) [][ points]
```

With `regmatch()` the captures belong to the macro that called it and last until
it ends. **Use them in a later command**, not in the same command line as the
`regmatch` call: a command is substituted before it runs. The reliable form is
`/if`:

```tf
/def hit = /if (regmatch('(\w+) hits you for (\d+)', {*})) /echo [%P1] did [%P2], left [%PL], right [%PR]%; /else /echo no match%; /endif
/hit The orc hits you for 12 damage
/hit nothing here
```
```output
[orc] did [12], left [The ], right [ damage]
no match
```

(`%P` is destroyed with the macro. A `regmatch` inside a bare `/test` line does
not leave captures for later lines of the same macro.)

Groups that don't participate are empty, and alternatives number in order:

```tf
/def alt = /if (regmatch('(a)|(b)', 'b')) /echo [%P1] [%P2]%; /endif
/alt
/def named = /if (regmatch('(?<animal>cat|dog)s?', 'two dogs')) /echo [%P1] [%P0]%; /endif
/named
```
```output
[] [b]
[dog] [dogs]
```

Named groups work, but they are still read by number.

## Defaults that surprise people

### Smart case

If the pattern contains **no uppercase letters**, matching is case-insensitive.
If it contains any uppercase letter, it is case-sensitive. Force either with
`(?i)` or `(?-i)` at the start.

```tf
/def sc = /echo $[regmatch('orc', 'An ORC appears')] $[regmatch('Orc', 'An ORC appears')] $[regmatch('Orc', 'An Orc appears')] $[regmatch('(?i)Orc', 'An ORC appears')] $[regmatch('(?-i)orc', 'An ORC appears')]
/sc
```
```output
1 0 1 1 0
```

### `.` matches newlines (DOTALL)

Lines from the world rarely contain newlines, but strings in scripts can. `.`
matches *any* character, newline included, so there is no need for `(?s)`.

### `$` matches only at the very end (DOLLAR_ENDONLY)

Unlike Perl, `$` does not also match before a trailing newline.

```tf
/def nl = /echo $[regmatch('^a.b$', strcat('a', char(10), 'b'))] $[regmatch('^foo$', 'foo')] $[regmatch('^foo$', strcat('foo', char(10)))]
/nl
```
```output
1 1 0
```

The first is `1` (DOTALL), the second `1`, the third `0` (the trailing newline
stops `$` from matching).

### UTF-8 and Unicode

Patterns and text are UTF-8, and Unicode properties are on: `\w`, `\d`, `\b` and
case folding understand accented letters, `.` is one *character* not one byte,
and `\p{L}` classes work.

```tf
/def u = /echo $[regmatch('é', 'café')] $[regmatch('^\w+$', 'café')] $[regmatch('^.{4}$', 'café')] $[regmatch('\p{L}+', 'ñandú')]
/u
```
```output
1 1 1 1
```

### `{` is special

`{` starts a counted repeat (`\d{3}`). To match a literal brace, write `\{`.

## What is supported

Everything the usual PCRE2 syntax offers for matching text: character classes and
POSIX classes (`[[:alpha:]]`), anchors and `\b`, greedy/lazy/possessive
quantifiers, non-capturing and named groups, alternation, backreferences
(`\1`), lookahead and lookbehind, atomic groups, inline options such as `(?i)`.

```tf
/def feat = /echo $[regmatch('^\d{3}-\d{4}$', '555-1234')] $[regmatch('\bcat\b', 'concat cat')] $[regmatch('[[:alpha:]]+', '123abc')] $[regmatch('(?<=\$)\d+', 'cost $45')] $[regmatch('foo(?=bar)', 'foobar')] $[regmatch('foo(?!bar)', 'foobar')] $[regmatch('(\w)\1', 'hello')] $[regmatch('^(?:ab)+$', 'ababab')] $[regmatch('(?>a+)b', 'aaab')] $[regmatch('a++b', 'aaab')]
/feat
```
```output
1 1 1 1 1 0 2 1 1 1
```

## What is not supported

This engine's regexp implementation deliberately leaves out:

* **recursion and subroutine calls**: `(?R)`, `(?1)`, `(?&name)`;
* **backtracking control verbs**: `(*SKIP)`, `(*FAIL)`, `(*COMMIT)`...;

Other exotic PCRE2 features not listed under "supported" should be treated as unavailable until you have tried them.

A pattern using them is rejected with an error, and the match fails:

```tf
/def bad = /echo $[regmatch('(a(?1)?b)', 'aabb')]
/bad
```
```output
0
```

(The reference client's own PCRE2 build accepts these. If you port a script
that uses them, rewrite it. See [FINDINGS](FINDINGS.md) for details.)

## Pitfalls

* **Forgetting the anchors.** `-mregexp -t'orc'` fires on *any* line containing
  "orc", including "forced".
* **Uppercase makes it case-sensitive.** A `-t'You'` regexp ignores case only if
  you write `(?i)`.
* **Using `%P1` in the same command as the `regmatch` that sets it.** It is
  substituted too early. Test in an `/if`, then use it in the body.
* **Unescaped `{`, `(`, `.`, `+`, `?`.** They are operators.
* **Expecting `*` to be "anything".** In a regexp it repeats the previous item;
  `.*` is "anything".
* **Catastrophic patterns** such as `(a+)+$` can run for a long time on a long
  non-matching line. Embedders can bound script work with the instruction limit.

## See also

[Patterns](Patterns.md) ·
[Triggers](Triggers.md) ·
[Expressions](Expressions.md) ·
[Substitution](Substitution.md#positional-parameters)
