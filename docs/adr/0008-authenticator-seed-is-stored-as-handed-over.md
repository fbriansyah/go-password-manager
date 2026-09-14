# The Authenticator Seed is stored exactly as the service handed it over

A TOTP Field's value is whatever the user pasted when two-factor was enabled:
either a bare base32 secret, read with RFC 6238 defaults (SHA1, six digits,
thirty seconds), or a full `otpauth://totp/` URI, whose `digits`, `period` and
`algorithm` parameters are honoured. Nothing is normalised on the way in — a
URI is not reduced to its secret, and a secret is not wrapped in a URI — and the
value is parsed every time a Code is derived. The alternatives were to keep only
the secret, which cannot represent the services that use eight digits or
SHA256, or to require a URI, which makes the user build one by hand out of the
sixteen characters most services show under "can't scan?", and repeats a title
and issuer that Meta already carries. Storing the input as given keeps `Field`
a plain string, keeps the file format unchanged, and asks the user to know
nothing about TOTP beyond copy and paste.

## Consequences

The `label` and `issuer` inside a URI are ignored for display — Meta and the
Field label are the only names a Secret has — and the URI's `secret` parameter
is the only part that is genuinely secret, but the whole value is treated as
one: hidden in the detail pane until revealed, never copied as-is. Because the
value is not normalised, a malformed seed is caught by validation at save time
rather than quietly rewritten; a file edited by hand or written by a newer
version can still hold one, so rendering and copying reject it again instead of
trusting the file.
