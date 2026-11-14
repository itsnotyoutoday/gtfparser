# Integration notes for the terminal builtins

The terminal work (`inputbuf.go`, `keys.go`, `status.go`, `terminal.go`) was
written without editing `interp.go`, `special.go` or the other files another
engineer is rewriting. Two small patches make it exact instead of
poll-based and field-based. Both are optional; everything works and is tested
without them.

## 1. Immediate variable-change notification (`OnVarChange`)

Today `OnVarChange` callbacks, the `var` event, the status line's reaction to
`%status_fields`/`%status_height`/`%more`/`%visual`/`%isize` changes and
`FitScreen` run when the interpreter next enters a terminal entry point
(`KeyPressed`, `StatusRows`, `kb*()`, `/dokey`, `/input`, `TermSettings`) or
when the client calls `PollVars()`. The effect of a bad value (for example
`/set status_height=7`) is therefore corrected a moment after the assignment
instead of rejecting it.

Exact patch for `special.go`, in `setGlobal`, replacing the final assignment:

```go
	old := ""
	if o, ok := in.globals[name]; ok {
		old = o.String()
	}
	in.globals[name] = v
	in.varChanged(name, old, v.String())
	return true
```

(`varChanged` is defined in `terminal.go`; it updates the poll snapshot, so a
direct report is never reported again by `PollVars`.) For `status_height > 6`
the callback restores the old value and warns, exactly like tf's
`ch_status_height`; to make `/set` return false in that case, check
`in.globals[name]` against `v` after calling `varChanged`, or move the
`status_height <= 6` test into `varSpec.coerce`.

`/unset` of a watched variable is also only seen by `PollVars`; to report it
immediately add `in.varChanged(name, old, "")` to `cmdUnset`.

## 2. State as a field instead of the weak registry

`ts()` looks the per-interpreter state up in a mutex-protected map keyed by a
`weak.Pointer[Interp]`, cleaned by `runtime.AddCleanup`. If you would rather
carry it in the struct, add to `Interp`:

```go
	term *termState
```

and replace `ts()` with

```go
func (in *Interp) ts() *termState {
	if in.term == nil {
		s := &termState{
			more:    map[string]*MoreState{},
			prompts: map[string]string{},
			echoOff: map[string]bool{},
			watch:   map[string]bool{},
		}
		s.hist.reset()
		s.st.init()
		in.term = s
	}
	return in.term
}
```

then delete `termMu`, `termStates`, `ReleaseTerminal`, the weak/runtime imports
and `TestTerminalStateIsCollected`.

## 3. Things in other files that interact with this work

* `macro.go`: `/def -B name` is recorded in `Macro.Keyname` and honoured by
  `Bindings()`; nothing else is needed. tf refuses `/bind` of a sequence that
  is a prefix of (or prefixed by) an existing binding ("is prefixed by an
  existing keybinding"); `cmdBind`/`Define` do not, and `KeyPressed` copes by
  preferring the longest match.
* `macro.go`: `/def -ag` gags (`Macro.Gag` mirrors the gag attribute), so a
  trigger can gag a line passed through `fake_recv()` or `ProcessLine`; the
  terminal code honours `ProcessLine`'s return value.
* `expr.go`: builtin commands are not callable with function syntax
  (`status_add("...")`, `localecho()`), which tf allows; the differential
  corpora avoid it.
* `registry.go` is not touched: `keycode` was removed from `zbuiltins.go`
  (it now lives in `keys.go` with the real name check) and the commands and
  functions of this work register themselves from `init()` like all others.
* `host.go` is unchanged: the new optional interfaces (`MoreHost`,
  `WindowHost`, `LineHost`, `PromptHost`, `StatusHost`) are declared next to
  their use in `terminal.go` to keep the merge trivial. Move them into
  `host.go` if you prefer them all in one place.
