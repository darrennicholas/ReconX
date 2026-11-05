<script setup lang="ts">
defineProps<{ kind: 'batch' | 'exception' | '' }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <Transition name="modal">
    <div v-if="kind" class="detail-popup-backdrop" @click.self="emit('close')">
      <section class="detail-popup" role="dialog" aria-modal="true" :aria-label="kind === 'batch' ? 'Detail batch' : 'Detail exception'">
        <header class="detail-popup-header">
          <div>
            <h2>{{ kind === 'batch' ? 'Detail Batch' : 'Detail Exception' }}</h2>
            <p>{{ kind === 'batch' ? 'Ringkasan dan tindakan batch rekonsiliasi' : 'Informasi dan penanganan selisih' }}</p>
          </div>
          <button type="button" class="close-button" data-no-loading aria-label="Tutup detail" @click="emit('close')">×</button>
        </header>
        <div class="detail-popup-body"><slot /></div>
        <footer class="detail-popup-footer"><button type="button" class="button secondary" data-no-loading @click="emit('close')">Tutup</button></footer>
      </section>
    </div>
  </Transition>
</template>
