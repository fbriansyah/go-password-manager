# Design

Aplikasi CLI TUI untuk menyimpan password, dan generate password berbasis file. Setiap content file secret di encrypt menggunakan Assymetric Encription (Public, Private Key) dam Master Password.

## TUI
TUI (Terminal UI) yang digunakan menggunakan library dari Charmbracelet:
- https://github.com/charmbracelet/bubbletea
- https://github.com/charmbracelet/bubbles
- https://github.com/charmbracelet/lipgloss

## Config
Format config menggunakan Yaml
```yaml
PUBLIC_KEY_PATH: ''
PRIVATE_KEY_PATH: ''
```

Keterangan:
- PUBLIC_KEY_PATH: Path yang menunjukkan public key file
- PRIVATE_KEY_PATH: Path yang menunjukkan private key file

## Format Data
Format data yang disimpan dan diencrypt berupa json
```json
{
    "meta": {
        "title": "Facebook",
        "description": "Facebook Credential",
        "tags": ["app", "facebook"]
    },
    "fields": [
        {"type": "tx", "label": "Username", "value": "username@mail.con"},
        {"type": "ps", "label": "Password", "value": "password123"},
        {"type": "tx", "label": "Website", "value": "http://facebook.com"},
        {"type": "ta", "label": "Note", "value": "this is text area note"}
    ]
}
```

## Phase
1. Setup Project and install dependency.
   1. TUI Libs
   2. Cobra, Viper
2. Load Current folder items and render in TUI
3. Manipulate file and encrypt file