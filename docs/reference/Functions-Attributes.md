# Functions: Attributes

Text on the screen carries *display attributes*: colour, bold, underline,
reverse and a few tf-specific flags such as "gag" or "no log". These
functions convert between three spellings of the same styled text:

- an internal styled string (what a trigger sees; invisible when printed raw),
- the **`@{...}` form**, readable and editable text such as `@{BCred}Danger@{n}`,
- the **terminal-code form** (ANSI escape sequences) a MUD sends.

Attribute lists use the letters `n` (none), `u` underline, `r` reverse, `B` bold,
`b` bell, `h` hilite, `x` exclusive, `g` gag, `G` nohistory, `L` nolog, `A`
noactivity, and `Ccolor` for a named colour (`Cred`, `Cgreen`, `Crgb520`, ...).
Separate items with commas where it would be ambiguous.

All the examples on this page are marked unverified (`tf-noverify`) because
these functions may be stubs in a given build (see [Status](Status.md)); the
output shown comes from running the same lines in real tf. The attribute codes
are shown with `encode_attr`, since the raw styled text is not printable.

## decode_attr(s [, attrs [, inline]])

**What it is for:** turning text with `@{...}` markers into truly styled text, so a macro can build coloured output, or applying one attribute to a whole string.

**Syntax:** `decode_attr(s1)`, `decode_attr(s1, s2)`, `decode_attr(s1, s2, f)`.

**Returns:** `s1` with its `@{attr}` markers interpreted as display attributes (as `/echo -p` does). If `s2` is given, those attributes are applied to the whole string (like `/echo -as2`). If `f` is 0 or "off" the `@{}` markers are *not* interpreted; this lets you apply `s2` to text that happens to contain `@{`.

```tf-noverify
/echo [$[strlen(decode_attr("@{BCred}Danger@{n}"))]]
/echo [$[encode_attr(decode_attr("hello", "Cgreen"))]]
/echo [$[decode_attr("a@@b")]]
```
```output
[6]
[@{Cgreen}hello@{n}]
[a@b]
```

**Gotchas:** markers take no space in the result (the 6 above is just `Danger`); `@@` stands for a literal `@`. A bad attribute name gives an error such as `invalid display attribute 'e'`. Typical use: `/echo -p @{BCred}...` does the same directly; use `decode_attr` when you need the styled string as a *value* (for `substitute()`, for `strcat`, or to store in a variable).

**Status:** see [Status](Status.md).

## encode_attr(s)

**What it is for:** the reverse of `decode_attr`: making the style of a styled string visible as `@{...}` markers, so you can log it, compare it, or print it for debugging.

**Syntax:** `encode_attr(s)`.

**Returns:** `s` with its attributes written out in `@{attr}` form; an unstyled string comes back unchanged (with a literal `@` doubled).

```tf-noverify
/echo [$[encode_attr("plain text")]]
/echo [$[encode_attr(decode_attr("@{BCred}Danger@{n} ahead"))]]
/echo [$[encode_attr(decode_attr("@{u}under@{n}"))]]
```
```output
[plain text]
[@{BCred}Danger@{n} ahead]
[@{u}under@{n}]
```

**Gotchas:** the order of attributes in the output is not documented and may change between tf versions; compare with `strcmpattr` rather than by string equality on the encoded text.

**Status:** see [Status](Status.md).

## strip_attr(s)

**What it is for:** getting the plain words of a coloured line, so a trigger or a string comparison is not confused by styling.

**Syntax:** `strip_attr(s)`.

**Returns:** `s` with all display attributes removed.

```tf-noverify
/echo [$[strip_attr(decode_attr("@{BCred}Danger@{n} ahead"))]]
/echo [$[strip_attr("plain")]]
```
```output
[Danger ahead]
[plain]
```

**Gotchas:** it removes attributes, not `@{...}` text; for raw text containing `@{` use `decode_attr` first.

**Status:** see [Status](Status.md).

## strcmpattr(s1, s2)

**What it is for:** comparing two strings including their styling: "is this really the same coloured text?".

**Syntax:** `strcmpattr(s1, s2)` returns an integer.

**Returns:** 0 only when both the text and the attributes are equal; non-zero otherwise (the sign carries no documented meaning for differing attributes). It is the same as `strcmp(encode_attr(s1), encode_attr(s2))`.

```tf-noverify
/echo [$[strcmpattr("abc", "abc")]]
/echo [$[strcmpattr(decode_attr("@{B}abc"), "abc") != 0]]
/echo [$[strcmp(decode_attr("@{B}abc"), "abc")]]
```
```output
[0]
[1]
[0]
```

**Gotchas:** plain `strcmp` looks at text only (last line: bold `abc` equals plain `abc`).

**Status:** see [Status](Status.md).

## encode_ansi(s)

**What it is for:** producing the escape-sequence form of styled text, for writing coloured text to a file, a pipe or a terminal that tf is not drawing for you.

**Syntax:** `encode_ansi(s)`.

**Returns:** `s` with attributes written as terminal control codes: plain ANSI, the 16-colour ISO 6429 extension, and xterm 256-colour codes.

```tf-noverify
/echo [$[replace(char(27), "<ESC>", encode_ansi(decode_attr("@{Cred}x@{n}")))]]
/echo [$[replace(char(27), "<ESC>", encode_ansi(decode_attr("@{BCyellow}Warn@{n}")))]]
```
```output
[<ESC>[31mx<ESC>[m]
[<ESC>[1;33mWarn<ESC>[m]
```

**Gotchas:** the example replaces the ESC character with `<ESC>` only so that the codes are visible on screen. Unstyled text is returned unchanged.

**Status:** see [Status](Status.md).

## decode_ansi(s)

**What it is for:** the entry point for text that arrives with raw ANSI codes (from a log, a file, a socket you read yourself): it becomes properly styled tf text.

**Syntax:** `decode_ansi(s)`.

**Returns:** `s` with ANSI, ISO 6429 16-colour and xterm 256-colour control sequences turned into display attributes. Any attributes `s` already had are not copied; if `%expand_tabs` is on, tabs become spaces according to `%tabsize`.

```tf-noverify
/echo [$[encode_attr(decode_ansi(strcat(char(27), "[1;31mred", char(27), "[0m done")))]]
/echo [$[strlen(decode_ansi(strcat(char(27), "[32mgreen", char(27), "[0m")))]]
/echo [$[encode_attr(decode_ansi(strcat(char(27), "[38;5;208morange", char(27), "[0m")))]]
```
```output
[@{BCred}red@{n} done]
[5]
[@{Crgb520}orange@{n}]
```

**Gotchas:** the escape bytes are removed from the length (5 above, just `green`). 256-colour codes are mapped to tf's `rgbRGB` colour names (`208` becomes `rgb520`).

**Status:** see [Status](Status.md).
