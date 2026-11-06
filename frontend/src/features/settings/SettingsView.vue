<script setup lang="ts">
import GlobalDropdown from '../../components/GlobalDropdown.vue'
import { roleCards } from './roles'
defineProps<{ role?: string }>()
const emit = defineEmits<{ save: []; users: [] }>()
const activeTab = defineModel<string>('tab', { required: true })
const timezoneSetting = defineModel<string>('timezone', { required: true })
const maxImportSetting = defineModel<number>('maxImport', { required: true })
const csvDelimiterSetting = defineModel<string>('delimiter', { required: true })
</script>

<template>
        <section class="settings-page">
          <nav class="settings-nav panel" aria-label="Kategori pengaturan">
            <button v-for="s in ['Umum','Parameter Matching','Integrasi','Notifikasi','Manajemen Role']" :key="s" :class="{ selected: activeTab === s || (activeTab === 'All' && s === 'Umum') }" @click="activeTab = s">{{ s }} <span>›</span></button>
          </nav>
          <article class="panel settings-card">
            <div class="panel-heading"><div><h2>{{ activeTab === 'All' ? 'Umum' : activeTab }}</h2><p v-if="activeTab === 'Umum' || activeTab === 'All'">Atur format dan batas penggunaan aplikasi.</p><p v-else-if="activeTab === 'Parameter Matching'">Aturan pencocokan yang digunakan saat ini.</p><p v-else-if="activeTab === 'Integrasi'">Sumber data yang tersedia pada versi ini.</p><p v-else-if="activeTab === 'Notifikasi'">Peristiwa yang ditampilkan pada notifikasi aplikasi.</p><p v-else>Ringkasan peran dan akses pengguna.</p></div></div>
            <section v-if="activeTab === 'Umum' || activeTab === 'All'" class="settings-section">
              <div class="settings-row"><div><b>Zona Waktu Default</b><small>Digunakan untuk tanggal batch dan audit log.</small></div><GlobalDropdown v-model="timezoneSetting" :disabled="role !== 'ADMIN'" class="form-control compact"><option value="Asia/Jakarta">Asia/Jakarta (WIB, UTC+7)</option></GlobalDropdown></div>
              <div class="settings-row"><div><b>Maksimum Ukuran Import</b><small>Batas ukuran file transaksi yang diterima.</small></div><GlobalDropdown v-model.number="maxImportSetting" :disabled="role !== 'ADMIN'" class="form-control compact"><option :value="5242880">5 MB</option><option :value="10485760">10 MB</option></GlobalDropdown></div>
              <div class="settings-row"><div><b>Format CSV</b><small>Delimiter untuk file transaksi.</small></div><GlobalDropdown v-model="csvDelimiterSetting" :disabled="role !== 'ADMIN'" class="form-control compact"><option value=",">Koma (,)</option></GlobalDropdown></div>
              <button v-if="role === 'ADMIN'" class="button primary settings-save" @click="emit('save')">Simpan Perubahan</button>
            </section>
            <section v-else-if="activeTab === 'Parameter Matching'" class="settings-section">
              <div class="matching-rule"><span class="rule-code">M01</span><div><b>Auto Exact Match</b><small>Reference, tanggal, arah, mata uang, dan nominal harus sama.</small></div><span class="readonly-chip">Aktif · Read-only</span></div>
              <div class="settings-row"><div><b>Toleransi Selisih Tanggal</b><small>Pengajuan manual memerlukan persetujuan Checker.</small></div><span class="settings-value">3 hari kalender</span></div>
              <p class="settings-note">Aturan matching ditetapkan pada versi aplikasi ini dan ditampilkan sebagai informasi.</p>
            </section>
            <section v-else-if="activeTab === 'Integrasi'" class="settings-section">
              <div class="settings-row"><div><b>Import CSV</b><small>Unggah data internal dan bank melalui menu Import Data.</small></div><span class="settings-value">Tersedia</span></div>
              <div class="settings-row"><div><b>Koneksi ke sistem eksternal</b><small>Pengaturan koneksi bank atau payment gateway belum tersedia.</small></div><span class="settings-value muted">Belum tersedia</span></div>
            </section>
            <section v-else-if="activeTab === 'Notifikasi'" class="settings-section">
              <div class="settings-row"><div><b>Notifikasi dalam aplikasi</b><small>Ditampilkan melalui ikon lonceng pada bagian atas halaman.</small></div><span class="settings-value">Tersedia</span></div>
              <div class="settings-row"><div><b>Aktivitas yang diberitahukan</b><small>Import gagal, hasil matching, pengajuan dan keputusan approval, serta penutupan batch.</small></div></div>
              <div class="settings-row"><div><b>Email dan saluran eksternal</b><small>Pengiriman notifikasi melalui saluran eksternal belum tersedia.</small></div><span class="settings-value muted">Belum tersedia</span></div>
            </section>
            <section v-else class="settings-section">
              <div v-for="role in roleCards" :key="role.name" class="settings-row"><div><b>{{ role.name }}</b><small>{{ role.description }}</small></div></div>
              <p class="settings-note">Daftar role ditetapkan oleh aplikasi. Admin dapat mengatur role setiap pengguna.</p>
              <button v-if="role === 'ADMIN'" class="button secondary settings-save" @click="emit('users')">Buka Users &amp; Access</button>
            </section>
          </article>
        </section>
</template>
