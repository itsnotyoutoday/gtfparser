# Working on the engine

This module is the single scripting core shared by every client. Rules:

1. **Clients only extend, never fork.** Client-specific behavior is attached
   through the public extension points (`RegisterCommand`, `RegisterFunction`,
   the host interfaces, `Policy`, `Limits`, `Options`). If a client needs more,
   the change is made here, generically, with no client-specific logic.
2. **Compatibility with real tf is the spec** (see *Compatibility policy* below). New behavior comes with a
   differential case in `testdata/cases` that passes against a real tf binary
   (`scripts/difftest.py`), or a unit test if real tf cannot observe it.
3. **API changes are additive.** Minor versions add; anything breaking gets a
   deprecation first, or bumps the minor version while v0.x, and is listed
   under `Breaking` in `CORE-CHANGES.md` with the migration.
4. **Tell the clients.** Add the entry to `CORE-CHANGES.md` in the same commit;
   clients adopt it by bumping their pinned version.
5. `go vet ./... && go test -race ./...`, the differential corpus and the
   golden library test (`GTF_TF_LIB=... go test .`) must pass before merging.

## Compatibility policy

gtfparser reproduces tf's results, formatting, warnings and errors exactly, including quirks that
follow from tf's own rules (evaluation order, float printing, documented conversions). It may deviate
**only** where tf's output is a defect of the *implementation*, not of the language: truncation from
mismatched integer widths, undefined behavior, platform- or locale-dependent results, or a crash or
hang. In such cases it returns the value the documented semantics imply (or, if that would need
unbounded resources, a clean error with a registered message) and keeps tf's conventions and wording
for every remaining error.

- The arbiter for "documented semantics" is tf-help, source comments and CHANGES. When unsure, **match
  and flag**.
- Behavior is pinned to one reference configuration: tf 5.0 beta 8, UTF-8 (WIDECHAR) build, 64-bit
  arm64 macOS, locale en_US.UTF-8.
- Every deviation is registered: a unit test, and a row in the *Deliberate differences* wiki page
  showing the script, what `TinyFugue:` prints and what `gtfparser:` prints, and why.
- Categories of deviation: implementation defect (e.g. integer narrowing), undefined or
  platform-dependent behavior (pinned to one deterministic value), resource ceiling (configurable,
  owned by the host), and crash/hang (fails cleanly). Behavior that merely differs but is defined is
  matched, not improved. Script-level infinite loops stay hangs (hosts add timeouts).
