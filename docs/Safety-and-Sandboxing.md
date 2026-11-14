# Safety and sandboxing

If you run scripts written by someone you do not fully trust (players, tenants,
a network service's users), decide what they may touch. The engine gives you
four dials; use as many as the situation needs.

| Dial | Controls | Set by |
|---|---|---|
| `/restrict` levels | shell, file and world access, tf's own model | script (raise only) or `WithRestriction` |
| `Policy` | your own veto over any privileged operation | `WithPolicy` |
| `Limits` and interrupts | how much CPU a script may use | `WithLimits`, `Interrupt()` |
| Disabling and overriding | which commands and functions exist at all | `DisableCommand`, tables |

## `/restrict`

tf's own restriction model, honoured everywhere it applies. The level can be
raised by a script but **never lowered**:

| Level | Name | Blocks |
|---|---|---|
| 0 | `NONE` | nothing |
| 1 | `SHELL` | `/sh`, `/quote !`, anything that runs an external command |
| 2 | `FILE` | also `/load`, `/save`, `/saveworld`, `/log`, `/lcd`, `/quote '`, `tfopen()` |
| 3 | `WORLD` | also creating new connections (`/addworld`, `/connect host port`) |

Start a sandbox at a level from Go:

```go
in := gtfparser.New(host, gtfparser.WithRestriction(gtfparser.RestrictFile))
in.Restriction() // 2
```

```tf
/restrict
```
```output
% restriction level: NONE
```

```tf-noverify
/restrict shell
/sh echo hi              % /sh: restricted
/restrict none           % /restrict: restriction level cannot be lowered
```

(An embedder that wants to ship a library of its own scripts into a restricted
interpreter loads them *before* raising the level, or loads them from Go.)

## `Policy`: your rules

`/restrict` is tf's model. Yours may be finer: this directory yes, that one no;
this host yes, others no. A `Policy` is consulted for every privileged
operation, **in addition to** `/restrict` (both must allow):

```go
type Policy interface {
    Allow(op gtfparser.Op, detail string) error   // nil = allowed
}

type sandbox struct{ root string }

func (s sandbox) Allow(op gtfparser.Op, detail string) error {
    switch op {
    case gtfparser.OpFileRead, gtfparser.OpFileWrite:
        if !strings.HasPrefix(detail, s.root) {
            return fmt.Errorf("outside the script directory")
        }
    case gtfparser.OpShell:
        return errors.New("no shell here")
    }
    return nil
}

in := gtfparser.New(host, gtfparser.WithPolicy(sandbox{"/srv/scripts/"}))
```

The operations are `OpShell`, `OpFileRead`, `OpFileWrite`, `OpDirChange` and
`OpNewWorld`; `detail` names the target (a path, a command line, a world). The
refusal text you return is what the script's author sees:
`% /load: outside the script directory`.

Your own commands take part automatically when you declare
`Needs: []gtfparser.Op{...}` in their [`CommandSpec`](Extending-the-Engine.md),
or call `c.Allow(op, "/name", detail)` yourself.

## Limits

```go
in := gtfparser.New(host, gtfparser.WithLimits(gtfparser.Limits{
    MaxInstr: 100_000, // statements per top-level execution
    MaxRecur: 50,      // nested macro calls
    MaxString: 1 << 20, // bytes strrep(), pad() or replace() may build (default 16 MiB)
}))
```

```tf-noverify
/set max_instr=1000
/def spin = /while (1) /test 1%; /done
/spin                    % spin: instruction count exceeded %max_instr (1000).
```

A runaway loop is stopped and reported (`% spin: instruction count exceeded
%max_instr (1000).`). tf's own variables `max_instr` and `max_recur` exist for
scripts to tune; your `Limits` are **ceilings**: a script can lower them but
never raise them above yours.

> `max_instr` counts statements. Real tf counts finer-grained internal
> instructions, so a loop is cut off at a somewhat different point than in the C
> client. The purpose, containing a runaway, is the same.

## Interrupt

```go
in.Interrupt()        // from any goroutine: the running script stops at the next statement
run.Interrupt()       // the same, through a Runner
```

Wire it to a "stop" button, a deadline, or a watchdog timer.

## Disabling and overriding

The most direct control: take the capability away.

```go
in.DisableCommand("sh")
in.DisableFunction("tfopen")
```

After `DisableCommand("sh")`, scripts that try it get `% /sh: disabled`.

## Other things to decide

- **The environment.** By default the process environment becomes script
  variables. Use `WithoutEnvironment()` when scripts are not yours.
- **What your own commands expose.** Everything you register is reachable. Give
  commands `Needs` so they honour restrictions, and validate arguments.
- **Regular expressions.** The regexp engine has a step limit (about ten
  million steps by default) so a pathological pattern cannot hang a match.
- **One interpreter per tenant.** Do not share an `Interp` between users; it
  holds one script world's variables and macros. Create one per user, session
  or script.

## What this is and is not

These controls are **guardrails for scripts, enforced inside the interpreter**.
They stop a script from asking for things you did not allow. They are not an
operating-system sandbox: for hostile code, run the whole process in a
container or a restricted account as well.
