# ReconX

ReconX adalah aplikasi **Bank Transaction Reconciliation & Exception Management** dengan arsitektur modular monolith. Backend memakai Go dan PostgreSQL; frontend memakai Vue 3, TypeScript, dan Vite.

> FSD MVP hanya mengizinkan data sintetis. Aplikasi ini tidak terhubung ke bank, tidak memindahkan dana, dan tidak memposting jurnal. Jangan unggah data nasabah atau bank nyata.

## Teknologi dan susunan kode

- **Backend:** Go dengan Gin sebagai HTTP runtime, PostgreSQL melalui pgx `database/sql` driver, satu proses API modular.
- **Frontend:** Vue 3, TypeScript, Vite, Vue Router untuk navigasi dan guard role, Pinia untuk state sesi, `fetch` API client dengan session cookie.
- **Modul bisnis:** `identity` (auth/RBAC), `banking` (rekening), `reconciliation` (batch, import, matching, laporan), dan `exceptions` (investigation, proposal, approval).
- **Platform:** konfigurasi, koneksi database, migration runner manual, audit append-only, HTTP envelope, CSRF dan idempotency.

```text
backend/cmd/api/                  API server
backend/cmd/migrate/              Jalankan migration secara eksplisit
backend/cmd/seed/                 Buat Admin awal jika belum ada user
backend/internal/modules/         Modul domain dan service
backend/internal/platform/        Database, HTTP, migration, audit
backend/internal/platform/migrate/001_initial.sql
backend/internal/platform/migrate/002_exception_candidates.sql
backend/internal/platform/migrate/003_ambiguous_references.sql
backend/examples/                 Fixture CSV sintetis
frontend/src/App.vue              Root Vue Router
frontend/src/Workspace.vue        UI ReconX dan alur API
frontend/src/router.ts            Route dan guard berdasarkan role
frontend/src/stores/session.ts    State sesi Pinia
frontend/src/api.ts               Session, CSRF, request ID, idempotency
```

## Role dan hak akses

| Aktivitas | Admin | Operator | Checker | Auditor |
| --- | --- | --- | --- | --- |
| Dashboard, rekening, batch dan hasil | Lihat | Lihat batch sendiri | Lihat | Lihat |
| Kelola pengguna dan rekening | Kelola | — | — | — |
| Buat/edit batch, import dan matching | — | Batch sendiri | — | — |
| Investigasi dan usulkan manual match | — | Batch sendiri / ditugaskan | — | — |
| Setujui/tolak dan tutup batch | — | — | Ya, bukan pengusul | — |
| Laporan dan ekspor | Ya | Batch sendiri | Ya | Ya |
| Audit log | Ya | — | Ya | Ya |
| Pengaturan | Kelola | — | Lihat | Lihat |

Admin mengelola akses dan konfigurasi. Operator menjalankan rekonsiliasi. Checker memeriksa keputusan dan menutup batch. Auditor meninjau hasil dan riwayat. Persetujuan memerlukan Checker yang berbeda dari pembuat pengajuan.

### Akun awal development

| Role | Email | Password awal | Keterangan |
| --- | --- | --- | --- |
| Admin | `admin@reconx.local` | `developer` | Dibuat oleh seeder lokal pada database kosong. |
| Operator | `operator@reconx.local` | `developer` | Menjalankan import dan rekonsiliasi. |
| Checker | `checker@reconx.local` | `developer` | Memeriksa dan menyetujui pengajuan. |
| Auditor | `auditor@reconx.local` | `developer` | Meninjau laporan dan audit. |
| Operator cabang | `operator.cabang@reconx.local` | `developer` | Memproses batch milik operator cabang. |
| Checker cadangan | `checker.cadangan@reconx.local` | `developer` | Memeriksa antrean persetujuan. |
| Auditor internal | `auditor.internal@reconx.local` | `developer` | Meninjau laporan dan audit. |

Seeder development membuat akun-akun tersebut serta rekening dan batch contoh untuk status draft, siap matching, investigasi exception, menunggu approval, dan selesai. Batch contoh dimiliki dua akun Operator agar cakupan akses dapat dicoba. Data juga mengisi transaksi, hasil matching, antrean approval, laporan, audit, dan notifikasi sesuai role. Seeder aman dijalankan kembali dan tidak mengubah password akun yang sudah ada. Password `developer` hanya untuk pengembangan lokal. Untuk membuat batch baru pada rekening contoh, pilih periode yang tidak bertindih dengan batch yang sudah ada.

Contoh tambahan mencakup perbedaan arah, referensi ganda (`AMBIGUOUS_MATCH`), exception yang ditugaskan ke operator lain, manual match yang disetujui dan ditolak, batch yang disetujui, ditolak dan dibatalkan, serta impor tidak valid. Data dibuat melalui layanan aplikasi agar hasil matching, approval, notifikasi dan audit tetap terhubung. Status sementara seperti `PROCESSING` tidak dibuat sebagai batch yang berhenti di tengah proses.

Untuk mencoba referensi ganda, gunakan `frontend/public/contoh/internal-ambiguous.csv` dan `frontend/public/contoh/bank-ambiguous.csv`: buat batch dengan periode yang mencakup tanggal di file, impor masing-masing sesuai sumber, lalu jalankan matching. File dapat dibuka dan disunting di Excel, kemudian disimpan sebagai CSV UTF-8. Referensi ganda sekarang diterima saat impor dan ditandai `AMBIGUOUS_MATCH` saat matching, perubahan dari aturan reference unik di FSD. File `.xlsx` belum didukung langsung.

## Alur bisnis (FSD §4)

1. Operator membuat batch untuk rekening dan periode tertentu. Backend menolak periode yang bertumpang-tindih pada rekening yang sama.
2. Operator mengunggah satu CSV `INTERNAL` dan satu `BANK`. Seluruh file divalidasi dulu; jika satu baris tidak sah, tidak ada transaksi dari file itu yang disimpan.
3. Setelah kedua sumber valid, batch menjadi `READY`. Operator menjalankan matching M01: reference, nominal, arah, mata uang, dan tanggal harus sama.
4. Batch masuk `REVIEW`. Hasil berupa pasangan otomatis dan exception seperti `AMOUNT_MISMATCH`, `DIRECTION_MISMATCH`, `DATE_VARIANCE`, `AMBIGUOUS_MATCH`, `INTERNAL_ONLY`, dan `BANK_ONLY`. Matching mencocokkan reference, tanggal, arah, mata uang, dan nominal; batch memakai mata uang IDR.
5. Operator dapat menugaskan exception dan menambahkan catatan. Manual match hanya dapat diajukan untuk `DATE_VARIANCE` yang memenuhi syarat, lalu harus diputuskan Checker yang berbeda dari maker.
6. Batch hanya bisa diminta untuk ditutup jika seluruh transaksi sudah cocok serta tidak ada exception atau approval yang tertunda. Checker menyetujui/menolak permintaan; hanya snapshot yang disetujui yang dapat ditutup. Batch tertutup tidak dapat diedit.
7. Setiap perubahan penting ditulis ke audit log. Create/import/matching/proposal/approval memakai `Idempotency-Key`; perubahan state batch memakai `If-Match` dan row version.

State utama batch: `DRAFT → READY → PROCESSING → REVIEW → PENDING_APPROVAL → APPROVED → CLOSED`. Import ulang/koreksi sumber menaikkan `source_version`, mengarsipkan hasil lama, dan membutuhkan matching baru. Batch `FAILED` dapat dicoba ulang; batch `DRAFT` dapat dibatalkan.

## Fitur MVP (FSD §5)

- Login/logout, session server, profil sesi, bootstrap Admin, manajemen user dan role tetap.
- Dashboard dengan filter periode/rekening, tren matching, distribusi status batch, dan top exception; rekening sumber serta daftar/detail batch.
- Upload CSV, validasi atomik, laporan error CSV, penyimpanan file private.
- Daftar transaksi dengan pencarian/filter/pengurutan dan paginasi, detail transaksi, exact matching, hasil run, dan exception workbench.
- Assignment, catatan, lampiran evidence private (PDF/PNG/JPEG/TXT maks. 5 MB), proposal manual match, maker-checker approval, submit/close batch.
- CSV laporan, audit log, notifikasi, dan pengaturan yang diizinkan FSD.
- Reports menyediakan pilihan ekspor ringkasan batch, transaksi, dan riwayat exception (termasuk catatan dan keputusan manual match) berdasarkan filter dan hak akses pengguna yang aktif.
- Detail batch, exception, transaksi, approval, dan audit dapat dibuka dari tombol aksi dalam modal. Detail audit menampilkan aktivitas dan ringkasan perubahan tanpa menampilkan kode teknis kepada pengguna.
- RBAC server-side: Admin, Operator, Checker, Auditor; Operator dibatasi ke batch miliknya; Checker tidak dapat menyetujui request yang dibuatnya sendiri.

## Database

Migration ada di source code, tetapi **API tidak menjalankannya saat startup**. Database lokal tidak berubah sampai Anda menjalankan `cmd/migrate`. Migration membuat schema MVP dan data referensi role/permission/settings; sedangkan `cmd/seed` membuat Admin awal dan, dalam development, akun role serta data contoh yang saling terhubung.

File migration berada di `backend/internal/platform/migrate`; riwayat penerapannya disimpan dalam tabel PostgreSQL `public.schema_migrations`.

Gunakan database PostgreSQL kosong yang memang disiapkan untuk ReconX. Perintah migration akan membuat tabel di database pada `DATABASE_URL`.

### Konfigurasi lokal

1. Salin contoh env dari root project dan sesuaikan URL database:

```powershell
cd C:\ReconX
Copy-Item .env.example .env
```

Untuk Laragon PostgreSQL (default port `5432`), ubah `.env` agar `DATABASE_URL` menunjuk ke database ReconX Anda, contohnya:

```text
DATABASE_URL=postgres://postgres@127.0.0.1:5432/reconx?sslmode=disable
```

Tambahkan password di URL jika akun PostgreSQL Anda memilikinya. Database `reconx` perlu dibuat sekali di server PostgreSQL, misalnya dari HeidiSQL atau `psql`. HeidiSQL hanya menampilkan tabel yang ada pada database/schema yang sedang dipilih.

2. Jalankan PostgreSQL. Dari terminal PowerShell backend, buat schema aplikasi secara eksplisit:

```powershell
cd C:\ReconX\backend
go run ./cmd/migrate
```

3. Tetapkan nilai Admin awal untuk sesi PowerShell ini, lalu buat user Admin. Untuk pengembangan lokal gunakan akun seed berikut:

```powershell
$env:BOOTSTRAP_ADMIN_NAME = 'ReconX Admin'
$env:BOOTSTRAP_ADMIN_EMAIL = 'admin@reconx.local'
$env:BOOTSTRAP_ADMIN_PASSWORD = 'developer'
go run ./cmd/seed
```

Password `developer` hanya untuk seed dalam lingkungan development; password pengguna lain tetap minimal 12 karakter. Seed aman untuk diulang: jika sudah ada user, Admin kedua tidak dibuat. Simpan password milik sendiri dan jangan commit `.env` atau kredensial.

4. Jalankan API dan frontend di dua terminal:

Terminal backend:

```powershell
cd C:\ReconX\backend
go run ./cmd/api
```

Terminal frontend:

```powershell
cd C:\ReconX\frontend
npm install
npm run dev
```

Buka <http://localhost:5173>. API berjalan di <http://localhost:8080>. Vite meneruskan `/api/*` ke API. Login memakai email/password Admin yang baru di-seed.

Untuk Docker PostgreSQL, `.env.example` memakai user/database `reconx`; jalankan `docker compose up -d postgres` hanya jika port 5432 tidak sedang dipakai PostgreSQL Laragon. Jangan jalankan dua server PostgreSQL pada port yang sama.

### Penugasan exception dan pengujian RBAC

Operator pemilik batch dapat menugaskan exception kepada operator aktif lain. Operator yang ditugaskan dapat memeriksa exception, melihat ringkasan batch terkait dan transaksi yang berkaitan dengan exception tersebut, serta melakukan tindakan investigasi sesuai statusnya. Operator lain tidak dapat mengubah exception tersebut. Penugasan baru dan proposal manual dibatasi ketika persetujuan masih menunggu; catatan dan bukti tetap dapat ditambahkan tanpa mengubah status persetujuan.

Jalankan tes backend dengan `go test ./...` dari direktori `backend`. Tes integrasi RBAC memerlukan database PostgreSQL khusus tes yang nama databasenya berakhir `_test`; tes membuat dan menghapus schema terisolasi di database tersebut:

```powershell
cd C:\ReconX\backend
$env:RECONX_RBAC_TEST_DATABASE_URL = 'postgres://postgres@127.0.0.1:5432/reconx_rbac_test?sslmode=disable'
go test ./internal/platform/httpserver -run TestRBACAndAssignedExceptionIntegration -count=1 -v
```

Sesuaikan URL dengan PostgreSQL lokal dan buat database tes terlebih dahulu. Tanpa variabel tersebut, tes integrasi dilewati.

## Endpoint API

Prefix: `/api/v1`

| Kelompok | Endpoint utama |
| --- | --- |
| Status/RBAC | `GET /health`, `GET /ready`, `GET /meta/rbac` |
| Session | `POST /auth/login`, `POST /auth/logout`, `GET /auth/me` |
| Users/rekening | `/users`, `/users/{id}`, `/users/{id}/reset-password`, `/bank-accounts` |
| Batch/import | `/batches`, `/batches/{id}`, `/batches/{id}/imports`, `/imports/{id}/errors`, `/imports/{id}/file` |
| Matching/transaksi | `/batches/{id}/match-runs`, `/batches/{id}/matches`, `/transactions` |
| Exception/approval | `/exceptions`, `/exceptions/{id}/actions`, `/exceptions/{id}/evidence`, `/exceptions/{id}/manual-match-proposals`, `/approvals` |
| Dashboard/operasi | `/dashboard/summary`, `/reports/reconciliations`, `/reports/reconciliations/export`, `/audit-logs`, `/notifications`, `/settings` |

Respons JSON menggunakan envelope `{ success, data, meta, error, request_id }`. Session cookie `HttpOnly`/`SameSite=Strict`; request perubahan memakai token CSRF. Import disimpan di `IMPORT_STORAGE_DIR` (default `backend/private-data/imports`), di luar folder public frontend.

## CSV sintetis

`backend/examples/internal.csv` dan `backend/examples/bank.csv` berisi fixture dengan header:

```csv
reference_id,date,direction,amount,currency
TRX001,2026-11-03,CREDIT,500000,IDR
```

Unggah satu file pada source `INTERNAL`, lalu file satu lagi pada source `BANK` untuk batch yang sama. Reference wajib diisi; reference ganda dapat menghasilkan exception ambiguous match. Tanggal mesti berada dalam periode batch; currency MVP adalah IDR.

## Validasi build

```powershell
cd C:\ReconX\backend
go test ./...
go build ./...

cd C:\ReconX\frontend
npm run build
```

Tes integrasi yang memerlukan PostgreSQL dijalankan dengan variabel khusus sebagaimana dijelaskan pada bagian pengujian RBAC. Build memeriksa kompilasi dan bundling aplikasi.
