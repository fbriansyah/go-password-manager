# Vault adalah folder kerja, bukan direktori terpusat

Password manager umumnya menyimpan semua kredensial di satu tempat tetap. Kami memilih sebaliknya: Vault secara default adalah `$PWD` tempat aplikasi dijalankan, sehingga Secret hidup bersama proyeknya dan bisa ikut di-commit ke repo. Konsekuensinya sengaja diterima: tidak ada "vault utama", menjalankan aplikasi dari folder yang salah menampilkan daftar kosong, dan pemuatan tidak rekursif — hanya file berekstensi vault di folder itu sendiri.

Flag global `-d` / `--directory` menunjuk Vault ke folder lain tanpa perlu `cd`. Path relatif diselesaikan terhadap `$PWD` lalu disimpan sebagai path absolut; folder yang tidak ada ditolak dengan pesan jelas, bukan dibuat diam-diam. Karena flag ini memindahkan Vault, override config `.gopm.yaml` dicari di folder Vault yang terpilih — bukan di `$PWD` — sehingga key yang dipakai selalu mengikuti Secret yang dibuka.
