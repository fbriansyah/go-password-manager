# Meta dienkripsi, tetapi judul sengaja dibocorkan lewat nama file

Seluruh isi Secret — termasuk Meta — dienkripsi, sehingga daftar di TUI hanya bisa ditampilkan setelah Unlock di awal sesi. Namun nama file diambil dari Slug judul dan ikut berubah saat judul diubah, demi folder yang terbaca lewat `ls` dan riwayat git yang masuk akal. Akibatnya judul tetap terlihat tanpa Unlock; yang benar-benar terlindungi hanyalah deskripsi, tag, dan seluruh Field. Ini kompromi yang disadari, bukan kelalaian.
