# Password Manager

A CLI/TUI application that stores credentials as encrypted files inside a working folder, opened with a private key protected by a master password.

## Language

**Vault**:
The folder of Secrets this session manages, not recursive; by default the folder the application was run from, and it can be pointed at another folder.
_Avoid_: database, store, repository

**Secret**:
One encrypted file inside a Vault holding one set of credentials, made of Meta and a series of Fields.
_Avoid_: entry, record, credential, item

**Meta**:
The part of a Secret that describes it — title, description, and tags — used to find the Secret, never used as a credential.
_Avoid_: header, metadata, info

**Field**:
One label/value pair inside a Secret, with a type that decides how the value is displayed and edited.
_Avoid_: attribute, property, entry

**Master Password**:
The secret phrase that opens the Identity. It never encrypts a Secret directly.
_Avoid_: passphrase, PIN, master key

**Identity**:
The private key that can decrypt Secrets; stored as a file encrypted with the Master Password.
_Avoid_: private key file, secret key

**Recipient**:
The public key Secrets are encrypted to; holding it alone is enough to create Secrets without being able to read them.
_Avoid_: public key file

**Unlock**:
The act of opening the Identity with the Master Password at the start of a session, which makes every Secret in the Vault readable.
_Avoid_: login, sign in, authenticate

**Field Type**:
The property of a Field that decides how its value is displayed, how it is edited, and what gets copied from it; the copied value is not always the stored value.
_Avoid_: kind, format, widget

**Slug**:
The form of a title that is safe as a file name, and the only part of a Secret readable without Unlock.
_Avoid_: filename, id, key

**Generator Policy**:
The rules that shape a generated password — how long it is and which character classes take part; it outlives any one password and is inherited by a Vault from the global configuration.
_Avoid_: settings, preferences, options, recipe

**Help**:
The full key reference for the screen that is currently showing, opened on demand and covering the screen until it is closed; distinct from the one-line hint every screen always carries.
_Avoid_: helper, cheat sheet, legend, hint
