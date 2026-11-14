# Patterns

A *pattern* is a description of text to match. tf uses them for triggers, hooks,
`/list` and `/purge` filters, and the `=/` operator. There are four styles, and
you choose one with the `-m` option or the `%matching` variable.

| Style | Matches when... | Case |
|---|---|---|
| `simple` | the text equals the pattern exactly | sensitive |
| `substr` | the text contains the pattern | sensitive |
| `glob` | the whole text matches a shell-like pattern (**the default**) | **insensitive** |
| `regexp` | a PCRE2 regular expression matches anywhere in the text | smart-case, see [Regular expressions](Regular-Expressions.md) |

`%matching` holds the default (initially `glob`). `/def -mregexp ...` overrides it
for one macro; `-P` (partial highlighting) implies `regexp`.

```tf
/def -msimple -t'exactly this' si = /echo simple hit
/trigger exactly this
/trigger exactly this and more
/def -msubstr -t'part' sub1 = /echo substr hit
/trigger a part of speech
/trigger apart
```
```output
simple hit
substr hit
substr hit
```

(`/trigger text` pretends `text` arrived from the world, which is how these
examples exercise triggers; see [Triggers](Triggers.md).)

## Glob patterns

| Pattern | Matches |
|---|---|
| `*` | any run of characters, including none |
| `?` | exactly one character |
| `[abc]`, `[a-z]` | one character from the set or range |
| `[^abc]` | one character not in the set |
| `{one\|two\|th*}` | one of several **words** |
| `\*`, `\?`, `\[`, `\{` | the literal character |
| anything else | itself, ignoring case |

A glob must match the **whole** line. To match a fragment, surround it with
`*`.

```tf
/echo [dog] vs [d?g]: $["dog" =/ "d?g"]
/echo [dg] vs [d?g]: $["dg" =/ "d?g"]
/echo [drug] vs [d*g]: $["drug" =/ "d*g"]
/echo [Mr.] vs [M[rs].]: $["Mr." =/ "M[rs]."]
/echo [Mx.] vs [M[rs].]: $["Mx." =/ "M[rs]."]
/echo [5] vs [[^a-z]]: $["5" =/ "[^a-z]"]
/echo [DOG] vs [dog]: $["DOG" =/ "dog"]
```
```output
[dog] vs [d?g]: 1
[dg] vs [d?g]: 0
[drug] vs [d*g]: 1
[Mr.] vs [M[rs].]: 1
[Mx.] vs [M[rs].]: 0
[5] vs [[^a-z]]: 1
[DOG] vs [dog]: 1
```

(The `=/` operator tests a string against a glob; see [Expressions](Expressions.md).)

### Word alternation: `{a|b}`

Braces match **whole words** (delimited by spaces or the ends of the string).
That is stricter than it looks: both sides of the braces must be a space,
a wildcard, or an edge of the pattern, and a `{...}` list may not contain spaces.

```tf
/echo [chupchup fehs] vs [{storm|chup*}*]: $["chupchup fehs" =/ "{storm|chup*}*"]
/echo [Storm jiggles] vs [{storm|chup*}*]: $["Storm jiggles" =/ "{storm|chup*}*"]
/echo [stormette jiggles] vs [{storm|chup*}*]: $["stormette jiggles" =/ "{storm|chup*}*"]
/echo [foop] vs [{foo}*]: $["foop" =/ "{foo}*"]
/echo [foo p] vs [{foo}*]: $["foo p" =/ "{foo}*"]
/echo [dead slug] vs [{d*g}]: $["dead slug" =/ "{d*g}"]
/echo [debug] vs [{d*g}]: $["debug" =/ "{d*g}"]
```
```output
[chupchup fehs] vs [{storm|chup*}*]: 1
[Storm jiggles] vs [{storm|chup*}*]: 1
[stormette jiggles] vs [{storm|chup*}*]: 0
[foop] vs [{foo}*]: 0
[foo p] vs [{foo}*]: 1
[dead slug] vs [{d*g}]: 0
[debug] vs [{d*g}]: 1
```

`{foo}*` does not match `foop`, but does match `foo p`. `{d*g}` matches the
single word `debug` but not the two words `dead slug`. `{}` matches the empty
string, which makes a word optional.

### Escaping

```tf
/echo [a*b] vs [a\\*b]: $["a*b" =/ "a\\*b"]
/echo [aab] vs [a\\*b]: $["aab" =/ "a\\*b"]
```
```output
[a*b] vs [a\*b]: 1
[aab] vs [a\*b]: 0
```

(Inside `/echo` text the backslash is itself halved by [substitution](Substitution.md#backslash-escapes),
hence the doubling in the second half of each line. In a trigger pattern given
to `/def -t'...'`, a single `\*` is what you write.)

## Which style for which job?

* **glob**: the right default. Lines like `You hit the orc.`, or
  `{Bob|Alice} tells you: *`. Fast and readable. Remember it is case-insensitive.
* **regexp**: when you need to capture parts of the line (`%P1`...), match
  repetition (`\d+`), or test a fragment of a line without surrounding stars.
  Remember that it is *not* anchored: `^`/`$` are yours to add.
* **simple**: exact fixed lines, like a prompt `> `.
* **substr**: "contains this word", cheaper than `*word*`.

Rough equivalences between globs and regexps:

| regexp | glob |
|---|---|
| `part of line` | `*part of line*` |
| `^entire line$` | `entire line` |
| `\bword\b` | `*{word}*` |
| `^(you\|hawkeye) ` | `{you\|hawkeye} *` |
| `foo.*bar` | `*foo*bar*` |
| `^[^ ]+ whispers,` | `{*} whispers,*` |
| `zoo+m` | *(none)* |

## `%matching` and the `-m` option

```tf
/echo default is [%matching]
/set matching=regexp
/def -t'^(\w+) waves' w1 = /echo regexp default: %P1
/set matching=glob
/trigger Ann waves
/def -mglob -t'* nods' w2 = /echo glob: %1
/trigger Bob nods
```
```output
default is [glob]
regexp default: Ann
glob: Bob
```

The style is fixed when the macro is *defined*, using whatever `%matching`
was then. Changing `%matching` later does not change existing macros.

## Pitfalls

* **Globs are case-insensitive; `=~` is not.** `"DOG" =/ "dog"` is true,
  `"DOG" =~ "dog"` is false.
* **Globs are anchored, regexps are not.** `-t'orc'` (glob) matches only a line
  that is exactly `orc`. Write `-t'*orc*'` or use `-msubstr`.
* **`{a|b}` is for whole words.** `{orc}*` does not match `orcs are here`.
* **Variables are not expanded in trigger patterns.** The pattern is whatever
  text you gave `/def`. See [Substitution](Substitution.md#why-you-write--inside-nested-definitions)
  for generating a pattern from a variable.
* **Special characters in regexps.** `.` `(` `*` `?` `+` `[` `{` `\` `$`
  `^` `|` need a backslash to mean themselves. The pattern `{` is special in
  regexps (see [Regular expressions](Regular-Expressions.md)).

## See also

[Regular expressions](Regular-Expressions.md) ·
[Triggers](Triggers.md) ·
[Hooks](Hooks.md) ·
[Expressions](Expressions.md)
