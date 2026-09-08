# A Vault is the working folder, not a central directory

Password managers usually keep every credential in one fixed place. We chose the opposite: a Vault is by default the `$PWD` the application was run from, so Secrets live next to the project they belong to and can be committed to its repository. The consequences are accepted deliberately: there is no "main vault", running the application from the wrong folder shows an empty list, and loading is not recursive — only files with the vault extension in that folder itself.

The global `-d` / `--directory` flag points the Vault at another folder without a `cd`. A relative path is resolved against `$PWD` and then stored as an absolute path; a folder that does not exist is refused with a clear message rather than created silently. Because the flag moves the Vault, the `.gopm.yaml` config override is looked for in the selected Vault folder — not in `$PWD` — so the keys in use always follow the Secrets being opened.
