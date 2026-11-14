# Functions: Strings

The string functions work on plain text. Positions are counted from 0, and
out-of-range positions are quietly clamped, so none of these functions fails
on a position that is too big. Every function here can be called inside
`$[...]`, in `/test`, in `/if` conditions and in any other expression. The last
section lists the library macros (from `stdlib.tf` and `textencode.tf`) that
are built from them.

For display attributes (colours, bold) see [Functions-Attributes](Functions-Attributes.md);
for pattern matching see [Functions-Regex-and-Eval](Functions-Regex-and-Eval.md).

## strlen(s)

**What it is for:** finding how long a string is, for padding, truncating or validating input.

**Syntax:** `strlen(s)` returns an integer.

**Returns:** the number of characters in `s` (not bytes: a UTF-8 character counts once).

```tf
/echo $[strlen("a dark cave")]
/echo $[strlen("")]
```
```output
11
0
```

**Gotchas:** a number is converted to its text first, so `strlen(1000)` is 4.

**Status:** see [Status](Status.md).

## substr(s, start [, len])

**What it is for:** cutting a piece out of a string, such as the first word of a line or the tail after a prefix.

**Syntax:** `substr(s, i1)` or `substr(s, i1, i2)`.

**Arguments/Returns:** the substring of `s` that begins at position `i1` and is `i2` characters long (the rest of the string when `i2` is left out). A negative `i1` or `i2` is counted from the end of the string.

```tf
/set room=The Wizard's Tower
/echo [$[substr(room, 4)]]
/echo [$[substr(room, 4, 6)]]
/echo [$[substr(room, 0, 1)]]
/echo [$[substr(room, 99)]]
```
```output
[Wizard's Tower]
[Wizard]
[T]
[]
```

**Gotchas:** a start past the end gives the empty string, never an error. Capitalizing a word is `strcat(toupper(substr(s, 0, 1)), substr(s, 1))`. Negative positions count from the end, and positions and lengths are in characters, not bytes.

```tf
/echo $[substr("hello", -2)] $[substr("hello", -3, 2)] $[strlen("héllo")] $[strcmp("B", "a")]
/echo $[strstr("hello", "l", 3)] $[tolower("HeLLo", 2)] $[toupper("hello", 2)]
```
```output
lo ll 5 -31
3 heLLo HEllo
```

**Status:** see [Status](Status.md).

## strcat(s...)

**What it is for:** gluing strings and numbers together inside an expression, since expressions have no concatenation operator.

**Syntax:** `strcat(s1, s2, ...)` with at least one argument.

**Returns:** all arguments converted to text and joined, with nothing between them.

```tf
/set hp=42
/set maxhp=120
/echo $[strcat("HP: ", hp, "/", maxhp)]
/echo $[strcat("pi is about ", 3.14159)]
```
```output
HP: 42/120
pi is about 3.14159
```

**Gotchas:** real tf rejects a call with no arguments at all ("expected between 1 and -1"); numbers are formatted the way tf prints them, so reals keep their trailing dot when whole (`strcat(2.0)` is `2.0`, but `strcat(sqrt(4))` is `2.`).

**Status:** see [Status](Status.md).

## strstr(s1, s2 [, start])

**What it is for:** finding where a word or phrase occurs inside a line, for example to cut a message off at a marker.

**Syntax:** `strstr(s1, s2)` or `strstr(s1, s2, i)`.

**Returns:** the position of the first occurrence of the whole string `s2` in `s1` at or after position `i` (default 0), or -1 if it is not there.

```tf
/echo $[strstr("You hit the orc hard.", "orc")]
/echo [$[strstr("You hit the orc hard.", "elf")]]
/echo $[strstr("abcabc", "bc")]
```
```output
12
[-1]
1
```

**Gotchas:** it matches the exact text, case included (use `regmatch` with `(?i)` for case-insensitive search). The optional `start` argument begins the search at that position; a negative start counts from the end.

**Status:** see [Status](Status.md).

## strchr(s1, s2 [, start])

**What it is for:** finding the first of several delimiter characters, such as `#` in a dbref or `:` in a prompt.

**Syntax:** `strchr(s1, s2)` or `strchr(s1, s2, i)`.

**Returns:** the position of the first character of `s1` that is any one of the characters in `s2`, or -1. Unlike `strstr`, `s2` is a *set* of characters, not a phrase.

```tf
/set dbref=(#123PML)
/echo $[strchr(dbref, "#")]
/echo $[0 + substr(dbref, strchr(dbref, "#") + 1)]
/echo $[strchr("north-east", "-_ ")]
/echo [$[strchr("abc", "xyz")]]
```
```output
1
123
5
[-1]
```

**Gotchas:** `0 + "123PML"` is 123 because tf reads the leading digits of a string when it needs a number; that is how the dbref idiom above works.

**Status:** see [Status](Status.md).

## strrchr(s1, s2 [, start])

**What it is for:** like `strchr` but from the right, for the *last* delimiter, for example the file extension in a name.

**Syntax:** `strrchr(s1, s2)` or `strrchr(s1, s2, i)`.

**Returns:** the position of the last character of `s1` that belongs to the set `s2`, searching backwards from position `i` (default the end), or -1.

```tf
/set file=maps/zone.7.txt
/echo $[strrchr(file, ".")]
/echo $[substr(file, strrchr(file, ".") + 1)]
```
```output
11
txt
```

**Gotchas:** as with `strchr`, `s2` is a character set. The optional position argument works as in `strchr`.

**Status:** see [Status](Status.md).

## strcmp(s1, s2)

**What it is for:** ordering or comparing strings, most often to test two names for equality.

**Syntax:** `strcmp(s1, s2)` returns an integer.

**Returns:** zero if the strings are equal, negative if `s1` sorts before `s2`, positive if after. The comparison is by character code, so upper case sorts before lower case.

```tf
/echo $[strcmp("apple", "apple")]
/echo $[strcmp("apple", "apple") == 0]
/echo $[strcmp("apple", "banana") < 0]
/echo $[strcmp("Zed", "abe") < 0]
```
```output
0
1
1
1
```

**Gotchas:** only the sign is documented, but tf returns the difference of the first differing bytes (`strcmp("B","a")` is -31) and so does this engine. For plain equality the operators `=~` (exact) and `==` on strings are usually shorter, and `=/` does glob matching.

**Status:** see [Status](Status.md).

## strncmp(s1, s2, n)

**What it is for:** comparing only the first `n` characters, for prefix tests.

**Syntax:** `strncmp(s1, s2, i)`.

**Returns:** like `strcmp`, but looking at no more than `i` characters of each string.

```tf
/echo $[strncmp("hello", "help", 3) == 0]
/echo $[strncmp("hello", "help", 4) == 0]
/echo $[strncmp("You say", "You", strlen("You")) == 0]
```
```output
1
0
1
```

**Gotchas:** the same sign-versus-difference note as `strcmp` applies.

**Status:** see [Status](Status.md).

## strrep(s, n)

**What it is for:** drawing rulers, bars and indentation.

**Syntax:** `strrep(s, i)`.

**Returns:** `s` repeated `i` times; the empty string when `i` is 0 or negative.

```tf
/echo [$[strrep("-=", 10)]]
/echo [$[strrep("x", 0)]]
/set hp=7
/echo [$[strrep("#", hp)]$[strrep(".", 10 - hp)]]
```
```output
[-=-=-=-=-=-=-=-=-=-=]
[]
[#######...]
```

**Gotchas:** the repeat count must not be huge; the engine, like tf, will happily build a very large string.

**Status:** see [Status](Status.md).

## replace(s1, s2, s3)

**What it is for:** substituting one piece of text with another everywhere in a string.

**Syntax:** `replace(s1, s2, s3)` (the argument order is *find, replacement, text*).

**Returns:** `s3` with every occurrence of `s1` replaced by `s2`. The search is literal text, not a pattern.

```tf
/echo $[replace("-", "+", "a-b-c")]
/echo $[replace("orc", "troll", "an orc, an orc")]
/echo $[replace("x", "y", "abc")]
```
```output
a+b+c
an troll, an troll
abc
```

**Gotchas:** the name suggests a count but the function returns the new string. Its library twin `/replace` (below) takes the same arguments as a command.

**Status:** see [Status](Status.md).

## pad([s, i]...)

**What it is for:** laying out columns of text, as in a `/who` table or a score sheet.

**Syntax:** `pad(s1, i1, s2, i2, ...)`: any number of string/width pairs.

**Returns:** each string padded with spaces to the absolute value of its width and all results joined. A positive width right-justifies the string (padding on the left); a negative width left-justifies it.

```tf
/echo [$[pad("Name", -8, "Lvl", 4, "HP", 6)]]
/echo [$[pad("Gandalf", -8, 30, 4, 412, 6)]]
/echo [$[pad("Bo", -8, 5, 4, 9, 6)]]
```
```output
[Name     Lvl    HP]
[Gandalf   30   412]
[Bo         5     9]
```

**Gotchas:** a string longer than its width is *not* cut off (`pad("abcdef", 3)` is `abcdef`), so table columns can slip. Use `substr` first if you need a hard limit. Numbers are accepted in place of strings.

**Status:** see [Status](Status.md).

## tolower(s [, n]) and toupper(s [, n])

**What it is for:** normalizing case before a comparison, or shouting a word.

**Syntax:** `tolower(s)`, `tolower(s, i)`, `toupper(s)`, `toupper(s, i)`.

**Returns:** `s` converted to lower (upper) case. If `i` is given only the first `i` characters are converted.

```tf
/echo $[toupper("level up!")]
/echo $[tolower("The ORC Attacks")]
/echo $[strcat(toupper(substr("gandalf", 0, 1)), substr("gandalf", 1))]
```
```output
LEVEL UP!
the orc attacks
Gandalf
```

**Gotchas:** conversion follows the locale, so accented letters convert under a UTF-8 locale. The optional count limits the conversion to the first characters: `tolower("HeLLo", 2)` is `heLLo`. Case mapping is by Unicode character, a [deliberate difference](../deliberate-differences.md) from tf's byte-wise mapping, which corrupts multibyte text.

**Status:** see [Status](Status.md).

## ascii(s)

**What it is for:** getting the numeric code of a character, for sorting, escaping, or building a checksum.

**Syntax:** `ascii(s)` returns an integer.

**Returns:** the character code of the first character of `s` (any character your locale allows, not only ASCII); 0 for the empty string.

```tf
/echo $[ascii("A")]
/echo $[ascii("abc")]
/echo $[ascii("")]
```
```output
65
97
0
```

**Gotchas:** only the first character counts.

**Status:** see [Status](Status.md).

## char(i)

**What it is for:** producing a character from its code, such as an ESC byte for raw terminal codes or a newline.

**Syntax:** `char(i)` returns a one-character string.

**Returns:** the character whose code is `i`. A code outside what the locale allows is forced into range.

```tf
/echo $[char(65)]$[char(66)]$[char(67)]
/echo $[strlen(char(27))]
```
```output
ABC
1
```

**Gotchas:** `char(0)` cannot be used to build strings; stick to 1 and up. Below 256 it returns the single byte, as tf does; above that it returns the UTF-8 character, a [deliberate difference](../deliberate-differences.md) (tf returns one byte, the code modulo 256).

**Status:** see [Status](Status.md).

## Library macros

These come from `stdlib.tf` (always loaded by a normal tf) and `textencode.tf`.
They are ordinary macros: call them as commands (`/first a b c`), use their
result with `$(/name args)`, or call them like functions inside expressions
(`first("a b c")`, which passes the whole argument as `%1`).

### /first, /rest, /last, /nth (Library)

**What it is for:** taking words from a space-separated list, such as the names in `/who`.

**Syntax:** `/first words`, `/rest words`, `/last words`, `/nth n words`.

**Returns:** `/first` the first word, `/rest` everything after the first word, `/last` the last word, `/nth n` the n-th word counting from 1 (empty if there is none).

```tf
/set party=Gandalf Bilbo Frodo Sam
/echo $(/first %party)
/echo $(/rest %party)
/echo $(/last %party)
```
```output
Gandalf
Bilbo Frodo Sam
Sam
```

**Gotchas:** they return text as a macro `/result`, so you reach it with `$(...)`, not by printing it.

**Status:** library macros from `stdlib.tf`; see [Status](Status.md) for the builtins they rest on.

### /split key = value (Library)

**What it is for:** breaking a `name = value` or `name value` line in two, for writing small configuration commands.

**Syntax:** `/split text`, which sets `%{P1}` to the part before the first `=` (trailing spaces trimmed) and `%{P2}` to the rest.

```tf
/def setopt = /split %*%; /echo option [%{P1}] gets [%{P2}]
/setopt wimpy  =  25 percent
/setopt noshout
```
```output
option [wimpy] gets [25 percent]
option [noshout] gets []
```

**Gotchas:** it is a thin wrapper over `regmatch`, so the `P` variables are *global* match results and the next `regmatch` overwrites them; read them straight away.

**Status:** library macro; see [Status](Status.md).

### /escape meta text (Library)

**What it is for:** putting a backslash in front of characters that would otherwise be special, so a user's text can be pasted into a command or pattern safely.

**Syntax:** `/escape chars text`: the first word is the set of characters to protect (the backslash itself is always protected).

```tf
/echo $(/escape ' a'b'c)
/echo $(/escape * 2*3)
```
```output
a\'b\'c
2\*3
```

**Gotchas:** inside a macro body a `%` in the text is substituted before `/escape` sees it, exactly as in tf, so protect it with `%%`.

**Status:** library macro; see [Status](Status.md).

### /replace old new text (Library)

**What it is for:** the command form of `replace()`, handy inside `$(...)`.

**Syntax:** `/replace old new text`; returns the changed text via `/result`.

```tf
/echo $(/replace - + a-b-c)
```
```output
a+b+c
```

**Gotchas:** `old` and `new` are single words (they are taken as `%1` and `%2`); use the `replace()` function for text with spaces.

**Status:** library macro; see [Status](Status.md).

### textencode(s) and textdecode(s) (Library)

**What it is for:** turning arbitrary text into a safe word of letters, digits and underscores (for macro names, variable names and file names) and back.

**Syntax:** `textencode(s)`, `textdecode(s)`; part of `textencode.tf`, which the engine provides built in.

**Returns:** `textencode` replaces every character that is not a letter or digit with `_code_` where `code` is the character number; `textdecode` reverses it.

```tf
/echo $[textencode("Bob's room")]
/echo $[textdecode(textencode("Bob's room"))]
```
```output
Bob_39_s_32_room
Bob's room
```

**Gotchas:** the encoding is not shortened, so long text gives long names.

**Status:** see [Status](Status.md).
