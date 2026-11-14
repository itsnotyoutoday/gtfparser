# Core changes

Every change to the engine that a client could notice is logged here, newest
first, in the same commit that makes it. Clients pin a version of this module
and update on purpose (`go get github.com/itsnotyoutoday/gtfparser@vX.Y.Z`),
reading this file for what changed. A change is never required to be taken
mid-task.

Format: `## vX.Y.Z (date) — summary`, then `Added` / `Changed` / `Fixed` /
`Breaking` (and what a client must do).

## Unreleased

## v0.0.1 (2026-11-14) — first extraction

The engine split out of the gtf client repo: interpreter, PCRE2-compatible
regexp engine, per-interpreter command/function registries, host capability
interfaces (WorldHost, Notifier, KeyHost, TermHost, HelpHost), Policy and
/restrict, Limits and Interrupt, the Runner actor.
