# age dipakai sebagai lapisan kripto, bukan kripto rakitan sendiri

Design.md menyebut "enkripsi asimetris", yang secara harfiah mengarah ke RSA + AES rakitan sendiri. Kami memakai `filippo.io/age` (X25519 + ChaCha20-Poly1305) karena format file, pembungkusan kunci, dan identity file terenkripsi passphrase sudah baku dan diaudit — kode kripto yang kami tulis sendiri menjadi nol. Efek sampingnya: format file terikat pada age, dan Secret tetap bisa dibuka dengan CLI `age` seandainya aplikasi ini tidak ada lagi.

Secret ditulis dalam bentuk armored ASCII agar aman melewati git dan copy-paste; isinya tetap berubah total setiap penyimpanan, jadi diff tidak pernah bermakna.
