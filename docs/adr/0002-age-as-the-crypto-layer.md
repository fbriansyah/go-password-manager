# age is the crypto layer, rather than cryptography assembled by hand

The design document said "asymmetric encryption", which read literally points at hand-rolled RSA + AES. We use `filippo.io/age` (X25519 + ChaCha20-Poly1305) because the file format, the key wrapping, and the passphrase-encrypted identity file are already standard and audited — the amount of cryptographic code we write ourselves drops to zero. The side effect: the file format is tied to age, and Secrets can still be opened with the `age` CLI should this application cease to exist.

Secrets are written as armored ASCII so they survive git and copy-paste; their contents change completely on every save regardless, so a diff is never meaningful.
