# Password Manager

Aplikasi CLI/TUI untuk menyimpan kredensial sebagai file terenkripsi di dalam folder kerja, dibuka dengan private key yang dilindungi master password.

## Language

**Vault**:
Folder berisi Secret yang dikelola sesi ini, tidak rekursif; secara default folder tempat aplikasi dijalankan, dan dapat ditunjuk ke folder lain.
_Avoid_: database, store, repository

**Secret**:
Satu file terenkripsi di dalam Vault yang berisi satu kumpulan kredensial, terdiri dari Meta dan sederet Field.
_Avoid_: entry, record, credential, item

**Meta**:
Bagian Secret yang mendeskripsikan dirinya — judul, deskripsi, dan tag — dipakai untuk menemukan Secret, bukan untuk dipakai sebagai kredensial.
_Avoid_: header, metadata, info

**Field**:
Satu pasangan label dan nilai di dalam Secret, dengan tipe yang menentukan cara nilai itu ditampilkan dan diedit.
_Avoid_: attribute, property, entry

**Master Password**:
Frasa rahasia yang membuka Identity. Tidak pernah mengenkripsi Secret secara langsung.
_Avoid_: passphrase, PIN, master key

**Identity**:
Private key yang bisa mendekripsi Secret; tersimpan sebagai file terenkripsi Master Password.
_Avoid_: private key file, secret key

**Recipient**:
Public key yang menjadi tujuan enkripsi Secret; cukup dimiliki untuk membuat Secret baru tanpa bisa membacanya.
_Avoid_: public key file

**Unlock**:
Peristiwa membuka Identity dengan Master Password di awal sesi, yang membuat seluruh Secret di Vault dapat dibaca.
_Avoid_: login, sign in, authenticate

**Field Type**:
Sifat sebuah Field yang menentukan cara nilainya ditampilkan, diedit, dan apa yang disalin darinya; nilai yang disalin tidak selalu sama dengan nilai yang tersimpan.
_Avoid_: kind, format, widget

**Slug**:
Bentuk judul yang aman dipakai sebagai nama file, dan satu-satunya bagian Secret yang terbaca tanpa Unlock.
_Avoid_: filename, id, key
