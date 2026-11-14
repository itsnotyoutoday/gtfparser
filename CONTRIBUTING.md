# Working on the engine

This module is the single scripting core shared by every client. Rules:

1. **Clients only extend, never fork.** Client-specific behavior is attached
   through the public extension points (`RegisterCommand`, `RegisterFunction`,
   the host interfaces, `Policy`, `Limits`, `Options`). If a client needs more,
   the change is made here, generically, with no client-specific logic.
2. **Compatibility with real tf is the spec.** New behavior comes with a
   differential case in `testdata/cases` that passes against a real tf binary
   (`scripts/difftest.py`), or a unit test if real tf cannot observe it.
3. **API changes are additive.** Minor versions add; anything breaking gets a
   deprecation first, or bumps the minor version while v0.x, and is listed
   under `Breaking` in `CORE-CHANGES.md` with the migration.
4. **Tell the clients.** Add the entry to `CORE-CHANGES.md` in the same commit;
   clients adopt it by bumping their pinned version.
5. `go vet ./... && go test -race ./...`, the differential corpus and the
   golden library test (`GTF_TF_LIB=... go test .`) must pass before merging.
