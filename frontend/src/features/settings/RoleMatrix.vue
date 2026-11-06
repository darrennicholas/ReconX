<script setup lang="ts">
import { roleCards, roleMatrixRows } from './roles'
</script>

<template>
        <section class="panel role-matrix-panel"><div class="panel-heading"><div><h2>Role &amp; Access Control</h2><p>Role tetap MVP · matrix baca-saja · bukan role builder dinamis.</p></div><span class="badge neutral">RBAC</span></div><div class="role-cards"><article v-for="role in roleCards" :key="role.name" class="role-card"><span :class="['role-symbol', role.tone]">{{ role.name.slice(0, 1) }}</span><div><b>{{ role.name }}</b><small>{{ role.description }}</small></div></article></div><div class="role-table-scroll"><table class="role-table"><thead><tr><th>Fitur</th><th>Admin</th><th>Operator</th><th>Checker</th><th>Auditor</th></tr></thead><tbody><tr v-for="row in roleMatrixRows" :key="row.feature"><td>{{ row.feature }}</td><td>{{ row.admin }}</td><td>{{ row.operator }}</td><td>{{ row.checker }}</td><td>{{ row.auditor }}</td></tr></tbody></table></div><div class="sod-note"><b>Segregation of duties</b><span>Checker tidak boleh menyetujui permintaan sendiri. Admin tidak mendapat bypass approval. Akun nonaktif kehilangan akses sesi.</span></div></section>
</template>
