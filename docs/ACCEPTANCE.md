# Cakupan acceptance FSD v1.0

Tes berikut menggunakan data sintetis. Pengujian integrasi memerlukan database PostgreSQL khusus tes dengan nama berakhiran `_test`; pengujian membuat dan menghapus schema terisolasi di dalam database tersebut. Jangan gunakan database aplikasi harian.

| Kriteria | Pemeriksaan yang tersedia |
| --- | --- |
| T01 | Login salah menghasilkan 401 tanpa session atau password di respons; akun nonaktif tidak dapat login. |
| T02, T17 | Operator ditolak dari endpoint pengguna; Auditor ditolak melakukan mutasi. |
| T03 | Permintaan approval oleh maker yang sama ditolak dan status tetap pending. |
| T04 | Batch dengan periode dan rekening bertumpuk ditolak, termasuk respons API 409. |
| T05 | Header CSV yang hilang dan data baris tidak valid ditolak; impor tidak valid tidak membuat transaksi. |
| T06 | File sumber yang sama tidak dapat diunggah ulang. |
| T07–T09 | Contoh 3+3, referensi berbeda, dan nominal berbeda diuji; selisih nominal memblokir pengajuan batch. |
| T10 | Selisih tanggal dapat diajukan; tes browser memeriksa persetujuan Checker. |
| T11 | Dua keputusan approval bersamaan menghasilkan tepat satu keberhasilan. |
| T12 | Exception aktif atau proposal pending memblokir pengajuan batch. |
| T13 | Revisi pada batch REVIEW membatalkan proposal manual yang pending dan mengarsipkan versi lama. |
| T14 | Approval dengan versi sumber yang tidak cocok ditolak. |
| T15 | Batch CLOSED menolak edit, impor, matching, dan pembukaan koreksi. |
| T16 | Ekspor batch tertutup tetap sama setelah upaya perubahan yang ditolak. |
| T18 | Klasifikasi deterministik 10.000 baris diuji; integrasi juga mengukur matching dan penyimpanan 10.000 baris per sumber. |

Untuk menjalankan tes dan benchmark dari PowerShell, ikuti petunjuk database tes di README, lalu jalankan:

```powershell
cd C:\ReconX\backend
$env:RECONX_RBAC_TEST_DATABASE_URL = 'postgres://postgres@127.0.0.1:5432/reconx_rbac_test?sslmode=disable'
go test ./... -count=1
go test ./internal/modules/reconciliation -run '^$' -bench BenchmarkClassifyTenThousand -benchtime=3x
```

Gunakan kredensial dan nama database tes sesuai instalasi lokal. Tes browser mencakup navigasi ke menu yang diizinkan untuk empat role, pembatasan akses Operator, layout desktop dan layar sempit, serta alur impor gagal, ambiguous match, matching, dan approval Checker.

Ada dua batas interpretasi FSD yang perlu dipertahankan saat menilai hasil: implementasi menerima referensi ganda agar dapat menghasilkan `AMBIGUOUS_MATCH`, sedangkan FSD v1.0 meminta penolakan saat impor. FSD juga menyebut revisi selama approval pending pada T13, sementara state machine §4.1 hanya mengizinkan revisi dari REVIEW; tes di atas memeriksa proposal manual pending pada REVIEW. Angka waktu matching bergantung pada mesin dan database dan bukan target layanan produksi.
