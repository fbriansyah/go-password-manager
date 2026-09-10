# Adaptive colors stay on Lip Gloss's compat shim, not the declarative LightDark helper

Bubble Tea v2 removed `lipgloss.AdaptiveColor` outright: Lip Gloss is now "pure"
and no longer queries the terminal itself, so choosing light or dark is meant
to happen in the Bubble Tea `Update` loop — listen for `tea.BackgroundColorMsg`,
keep an `isDark bool` on `Model`, and pick colors with `lipgloss.LightDark(isDark)`
at render time. Doing that here would have meant threading that bool as a
parameter through `unlock.View`, `formModel.View`, `detailModel.View`, and every
helper they call (`labeled`, `generatorView`, `statusLine`), since `styles.go`'s
styles are package-level `var`s built once, not values `Model` owns.

We used `charm.land/lipgloss/v2/compat.AdaptiveColor` instead: a drop-in the
Lip Gloss authors ship specifically for this migration, which resolves
`Light`/`Dark` off a package-level `compat.HasDarkBackground` it queries at
init — the same moment (and the same one-shot query) v1's `AdaptiveColor` did
it, so `styles.go` keeps `var styleTitle = lipgloss.NewStyle()...` unchanged
in shape.

## Consequences

The Vault's palette is fixed for the life of the process, decided before
Bubble Tea takes over the terminal — exactly v1's behavior, so no visual
regression. It will not react if the terminal's background changes mid-session
(a rare case Bubble Tea v2 newly supports via `tea.BackgroundColorMsg`, and
which this app has never handled). Moving to the declarative `LightDark` form
is still open later; it is a styles-layer change, not a key-handling one, and
does not need to happen alongside a bubbletea/bubbles/lipgloss version bump.
