# The clipboard tool is the only thing behind a seam

`internal/clipboard` makes a promise: a copied password leaves the system
clipboard after 30 seconds, unless the user has copied something else since, in
which case theirs is left alone. The TUI counts that promise down in the status
line. Nothing tested it. Coverage was 27.3%, the lowest in the repository, and
the single test only checked that copying fails when no clipboard tool is
installed — the one path reachable by setting `PATH` to an empty folder.

Nothing could reach the rest because there was nothing to stand in front of.
`detect`, `run` and `output` were package functions that called `exec.LookPath`
and `exec.Command` directly, so the only lever a test had was `PATH`, and the
only observable outcome was failure.

**The system clipboard is now a seam of three questions** — put a value in it,
read what is in it now, empty it — **and everything else in the module is a rule
written above it.** Which tool an operating system prefers is a rule. Whether
the clipboard still holds the value we put there is a rule. Both are exercised
with no clipboard, no tool and no process, on any machine.

**This is the opposite conclusion from docs/adr/0013, and deliberately so.**
There a seam under the Vault was refused because a Vault *is* the working
folder, so the only second adapter would ever have been a test fake. That test
does not fail here: this module already had three real adapters. `pbcopy`,
`wl-copy` and `xclip` are three genuinely different ways to hold a clipboard,
with different argv for copy, paste and clear — they were simply written as data
inside `detect()` rather than as something with a shape. The fake in the tests
is a consequence of the seam, not the reason for it.

**The operating system's name is an argument, not a call to `runtime.GOOS`.**
`open(goos, have)` takes both the system it is choosing for and a way to ask
whether a binary is installed. Without that, the `pbcopy` branch was
unreachable from any Linux machine, which is every machine this project is
built on. The preference order itself is unchanged from the old `detect()`;
moving it did not rewrite it.

**The seam is unexported, and so are the tests.** Nothing outside this package
supplies a clipboard: the three adapters that matter all live here. An exported
`Tool` interface with an exported constructor taking one would be surface that
only tests use — the same shape as `Available()`, which this change deletes for
exactly that reason. The tests are in `package clipboard` instead, which is
where the fake belongs.

**`scheduleClear` is left alone, on purpose.** It re-runs this binary as a
detached process, and it does not talk to a clipboard tool at all — it talks to
`os.Executable`, `exec.Command` and `syscall`. "Which tool holds the clipboard"
and "how a clear is scheduled for later" are two different questions and want
two different seams. The second one is worth its own decision, because its
shape depends on whether the delay stays a detached process; that is
deliberately still open.

What did change is that the *scheduling* is a field on the rules rather than a
direct call, so a copy that succeeds is observable at all: the tests can see
that a value reached the clipboard and that its fingerprint — never the value —
was handed to the schedule, and that a copy the tool rejected schedules nothing,
since a clear for a value that never arrived would run against whatever the user
copied instead. How the spawning is done is still written in one place and still
untested.

## Consequences

Coverage went from 27.3% to 63.8%, and the split is the point: every rule is at
100%, and what remains at 0% is exactly the `exec` adapter — `put`, `get`,
`wipe`, `run`, `output`, `scheduleClear`. Those need a real binary or the second
seam above; the number will not move again without one of the two.

Nine tests stand where one did, over rules nothing could reach before —
including both branches of the fingerprint guard and the `pbcopy` preference.

`Available()` is deleted. It had no production caller, and its answer is the
error `Copy` already returns. Note what it did *not* do: the TUI never asked it,
so `gopm` still offers a copy binding that only fails once it is pressed. That
was true before this change and is still true after it.

`ClearCommand` holds the name `clipboard-clear` in one place. It was previously
written once in `internal/clipboard` and once in `cmd`, with nothing connecting
them, so renaming either would have left the clipboard never clearing and no
test failing.
