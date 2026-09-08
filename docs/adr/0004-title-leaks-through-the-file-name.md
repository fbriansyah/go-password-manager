# Meta is encrypted, but the title leaks through the file name on purpose

Everything inside a Secret — Meta included — is encrypted, so the TUI list can only be shown after the Unlock at the start of a session. The file name, however, is taken from the Slug of the title and follows it when the title changes, for the sake of a folder that reads sensibly under `ls` and a git history that makes sense. The result is that titles remain visible without unlocking; what is genuinely protected is the description, the tags, and every Field. This is a conscious compromise, not an oversight.
