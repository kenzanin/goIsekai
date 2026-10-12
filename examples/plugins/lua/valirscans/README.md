# valirscans — NOT installed (skipped)

**Status: Tidak di-install — contoh saja (reference only).**

Alasan (2026-10-12, arahan user):
1. Site di-protect Cloudflare — request server-side diblokir/challenge, plugin tidak bisa search/detail stabil.
2. Tidak ada manga yang bisa diverifikasi — hasil grounding kosong/tidak konsisten, jadi genre support juga tidak dilanjutkan.

Isi direktori ini (`main.lua` + `logo.png`) dipertahankan sebagai contoh plugin JSON-API
(Next.js `/api/search?q=`) untuk referensi, bukan untuk di-copy ke `app_data/plugins/`.

Jangan install: jangan copy ke `app_data/plugins/valirscans/` dan jangan daftarkan di DB.
Kalau proteksi CF sudah longgar / konten sudah ada, grounding ulang dulu
(vocabulary genre + filter param) sebelum install.
