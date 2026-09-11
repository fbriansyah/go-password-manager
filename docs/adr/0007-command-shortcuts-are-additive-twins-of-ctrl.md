# Command shortcuts on macOS are additive twins of ctrl, never replacements

A macOS user expects ⌘S to save, so the obvious change is to swap the `ctrl+*`
bindings for `super+*` when `runtime.GOOS` is `darwin`. We deliberately did not:
Terminal.app and iTerm2 consume Command before the program inside them ever sees
it — ⌘N opens a window, ⌘C copies the selection, ⌘S saves a transcript — and
`ModSuper` only reaches a TUI through the kitty keyboard protocol, in a terminal
that both implements it and declines to claim Command for itself. Swapping would
therefore have left the majority of macOS users with five dead keys and no way
back. Instead the application answers to both: a single `shortcut(msg) string` in
`internal/tui/keys.go` rewrites a leading `super+` to `ctrl+` when `keyOS` is
`darwin`, and every `switch` keeps matching the canonical `ctrl+*` literals. Only
the string used for matching is rewritten — the original `tea.KeyPressMsg` still
reaches the text inputs untouched.

`ctrl+c` is excluded on purpose. It is a terminal convention rather than an
application binding, and Command-C means *copy* on macOS; wiring it to quit would
close the Vault for anyone trying to copy text off the screen.

## Consequences

Ctrl must never be removed from the macOS path — the ⌘ bindings are a bonus for
terminals that can deliver them, not the supported way in. That is also why the
help line reads `⌘/^s save` rather than `⌘s save`: a user whose terminal eats
Command has to be able to see the binding that still works. Because the
normalisation sits at the top of `handleKey` and `handleGeneratorKey` rather than
in the individual cases, any `ctrl+*` binding added later gets its ⌘ twin for
free. Terminal capability is never probed: `tea.KeyboardEnhancementsMsg` would
tell us the protocol is supported, but not that the terminal has let Command
through, so it would be a confident answer to the wrong question.
