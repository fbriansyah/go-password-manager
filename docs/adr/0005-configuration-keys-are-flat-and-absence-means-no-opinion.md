# Configuration keys are flat, and an absent key means no opinion

Configuration arrives in up to three layers — the built-in defaults, the global
`config.yaml`, and the `.gopm.yaml` beside the Secrets — and the Generator Policy
made the merge rule matter for the first time, because a boolean read from an
absent key is indistinguishable from one written as `false`. The rule is that a
layer only speaks about the keys it actually contains (`viper.IsSet`, never a
zero-value test), and every key is flat and SCREAMING_SNAKE — `GENERATOR_SYMBOLS`,
not a nested `GENERATOR:` block. Flat keys keep one merge rule for the whole file
instead of one for scalars and another for blocks, and they let the writer that
saves a Policy from the TUI edit a single line in place, leaving the user's
comments and any keys it does not recognise untouched — a nested block would have
needed a real YAML rewrite, which loses both.

## Consequences

A file that says nothing about a key inherits it, so defaults can be improved
later and reach users who never chose otherwise. This is why `gopm init` writes
only the two key paths: writing out the Generator Policy defaults would freeze
today's numbers into every configuration ever created.
