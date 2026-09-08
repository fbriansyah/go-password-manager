# Master Password melindungi Identity, bukan Secret

Master Password tidak pernah mengenkripsi Secret secara langsung. Ia hanya membuka file Identity (private key terenkripsi passphrase). Membaca Secret karena itu butuh dua hal: file Identity dan Master Password. Menulis Secret hanya butuh Recipient — sebuah mesin bisa menambah Secret tanpa pernah mampu membacanya. Properti menulis-tanpa-membaca inilah satu-satunya alasan desain ini asimetris; kalau properti itu dilepas, enkripsi simetris murni akan lebih sederhana.

Tidak ada mekanisme pemulihan: kehilangan Identity atau lupa Master Password berarti seluruh Secret hilang permanen. `gopm init` wajib menyatakan ini.
