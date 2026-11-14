# Functions: Protocol

Modern MUD servers talk to the client through telnet *subnegotiations*
carrying structured data: GMCP (JSON-like messages), MSDP (key/value
variables), ATCP (the older Achaea protocol) and a vendor-specific option 102.
tf exposes one function per protocol to **send** such a message to the server;
incoming messages are delivered through hooks of the same name (`GMCP`,
`MSDP`, `ATCP`, `OPTION102`, and `GMCP_LOGIN` after connecting) so a macro can
react with `/def -h"GMCP Char.Vitals *" ...`.

These functions need a live connection and a build of tf that has the protocol
enabled; the settings `/set gmcp on`, `/set msdp on` and so on (read when a
connection negotiates its options, so set them *before* connecting) tell the
server tf wants the data. All four return 1 if the message was sent and 0 if
there is nothing to send it to. In the reference build used for this manual
the protocols were compiled out, so the examples below are marked unverified
(`tf-noverify`) and the described behaviour follows the tf source and manual.
Sending needs a host that owns connections (capability `WorldHost`).

## gmcp(message [, world])

**What it is for:** sending a GMCP message such as a package request or a login message to the server, for status bars and automappers driven by server data.

**Syntax:** `gmcp(s1)` or `gmcp(s1, s2)`: `s1` is the message text (package name, a space, then the data), `s2` an optional world (default: the current one).

**Returns:** 1 once the subnegotiation (`IAC SB GMCP ... IAC SE`) was sent.

```tf-noverify
/test gmcp('Core.Supports.Set ["Char 1", "Room 1"]')
/def -h'GMCP_LOGIN *' gmcp_login = /test gmcp('Char.Login {"name":"Bob","password":"secret"}')
/def -mglob -h'GMCP Char.Vitals *' vitals = /echo vitals: %-1
```

**Gotchas:** the message is sent as given; tf does not build or validate the JSON. The `GMCP_LOGIN` hook fires after connecting and is the right place for a login message. Keep passwords out of macros you share.

**Status:** see [Status](Status.md). Host capability: `WorldHost`.

## msdp(command [, world])

**What it is for:** asking an MSDP server for variables or telling it which ones to report (health, room, opponent) so scripts can react without parsing text.

**Syntax:** `msdp(s1)` or `msdp(s1, s2)`. The command is written in a readable form: `NAME=value` for a variable, `[ ... ]` for a table, `{ ... }` for an array. tf encodes it into the binary MSDP frame.

**Returns:** 1 if sent, 0 if there is no connection.

```tf-noverify
/test msdp("LIST=COMMANDS")
/test msdp("REPORT={ HEALTH MANA ROOM_NAME }")
/def -mglob -h'MSDP *' msdp_show = /echo -- MSDP: %*
/def msdp = /test msdp({*})
```

**Gotchas:** incoming data arrives on the `MSDP` hook in the same readable syntax (`HEALTH=42`, `ROOM= [ VNUM=1234 NAME=The void ]`). Scalar values are not quoted, so a value containing spaces or brackets is not self-delimiting. The setting `msdp` must be on before connecting. `/msdp ...` (the macro defined last above) is the convenient typing form.

**Status:** see [Status](Status.md). Host capability: `WorldHost`.

## atcp(message [, world])

**What it is for:** the Achaea Telnet Client Protocol, the predecessor of GMCP, still used by some older servers for room and vitals data.

**Syntax:** `atcp(s1)` or `atcp(s1, s2)`.

**Returns:** 1 once the `IAC SB ATCP ... IAC SE` frame was sent.

```tf-noverify
/test atcp("Auth.Request ATCP")
/def -mglob -h'ATCP *' atcp_show = /echo atcp: %*
```

**Gotchas:** enable with `/set atcp on` before connecting. Prefer GMCP where the server offers both.

**Status:** see [Status](Status.md). Host capability: `WorldHost`.

## option102(message [, world])

**What it is for:** sending a message on telnet option 102, a vendor channel some servers use for game-specific data.

**Syntax:** `option102(s1)` or `option102(s1, s2)`.

**Returns:** 1 once the `IAC SB 102 ... IAC SE` frame was sent.

```tf-noverify
/test option102("hello")
/def -mglob -h'OPTION102 *' o102 = /echo option 102: %*
```

**Gotchas:** the bytes are exactly what you give: the function is a raw channel with no framing of its own. Setting `option102` on enables negotiation.

**Status:** see [Status](Status.md). Host capability: `WorldHost`.
