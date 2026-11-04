<script setup lang="ts">
import { onMounted, ref } from 'vue'

const apiStatus = ref<'checking' | 'online' | 'offline'>('checking')

onMounted(async () => {
  try {
    const response = await fetch('/api/v1/health')
    apiStatus.value = response.ok ? 'online' : 'offline'
  } catch {
    apiStatus.value = 'offline'
  }
})
</script>

<template>
  <main class="shell">
    <aside class="sidebar">
      <a class="brand" href="#" aria-label="ReconX home">
        <span class="brand-mark">R</span>
        <span>Recon<span class="brand-light">X</span></span>
      </a>
      <div class="workspace-label">WORKSPACE</div>
      <nav aria-label="Main navigation">
        <a class="nav-item active" href="#overview"><span class="nav-icon">◫</span> Overview</a>
        <a class="nav-item" href="#reconciliation"><span class="nav-icon">⇄</span> Reconciliation</a>
        <a class="nav-item" href="#exceptions"><span class="nav-icon">⚑</span> Exceptions <span class="nav-count">0</span></a>
        <a class="nav-item" href="#transactions"><span class="nav-icon">▤</span> Transactions</a>
      </nav>
      <div class="sidebar-bottom"><span class="avatar">RX</span><span><strong>ReconX Workspace</strong><small>Development</small></span><span class="more">···</span></div>
    </aside>

    <section class="main-panel">
      <header class="topbar"><div class="breadcrumb">Workspace <span>/</span> <strong>Overview</strong></div><div class="top-actions"><span class="environment"><i></i> DEVELOPMENT</span><button class="icon-button" aria-label="Notifications">♧</button><span class="user-avatar">AD</span></div></header>
      <div class="content" id="overview">
        <div class="page-heading"><div><p class="eyebrow">RECONCILIATION WORKSPACE</p><h1>Good morning, Admin <span>✦</span></h1><p class="subtitle">Here’s the latest activity across your reconciliation workspace.</p></div><button class="primary-button" disabled title="Transaction imports are not configured yet">＋ <span>Import transactions</span></button></div>

        <section class="status-banner">
          <div class="status-icon">✧</div><div class="status-copy"><strong>Your reconciliation workspace is ready to be configured</strong><span>Connect your first bank account and import transactions to get started.</span></div>
          <div class="api-indicator"><i :class="apiStatus"></i> API {{ apiStatus }}</div>
        </section>

        <div class="section-title"><div><h2>Workspace overview</h2><p>Activity summary for your workspace</p></div><button class="period-button">Last 30 days <span>⌄</span></button></div>
        <div class="metrics-grid">
          <article class="metric-card"><div class="metric-top"><span>Total transactions</span><span class="metric-icon blue">▤</span></div><div class="metric-value">—</div><div class="metric-foot">No transactions imported</div></article>
          <article class="metric-card"><div class="metric-top"><span>Matched</span><span class="metric-icon green">⇄</span></div><div class="metric-value">—</div><div class="metric-foot">No reconciliation runs yet</div></article>
          <article class="metric-card"><div class="metric-top"><span>Open exceptions</span><span class="metric-icon amber">⚑</span></div><div class="metric-value">—</div><div class="metric-foot">Nothing needs your attention</div></article>
          <article class="metric-card"><div class="metric-top"><span>Match rate</span><span class="metric-icon violet">◌</span></div><div class="metric-value">—</div><div class="metric-foot">Available after first reconciliation</div></article>
        </div>

        <section class="activity-card"><div class="activity-heading"><div><h2>Recent activity</h2><p>Latest events across your workspace</p></div><button class="text-button" disabled>View all <span>→</span></button></div><div class="empty-state"><div class="empty-illustration"><span>⇄</span></div><strong>No activity yet</strong><p>When you import transactions or run a reconciliation,<br />the latest updates will appear here.</p><button class="secondary-button" disabled>Get started <span>→</span></button></div></section>
        <footer>ReconX <span>·</span> Bank Transaction Reconciliation &amp; Exception Management</footer>
      </div>
    </section>
  </main>
</template>
