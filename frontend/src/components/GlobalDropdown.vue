<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useAttrs, useId, useSlots, type VNode } from 'vue'

defineOptions({ inheritAttrs: false })
const props = defineProps<{ modelValue?: string | number | boolean; disabled?: boolean; modelModifiers?: { number?: boolean } }>()
const emit = defineEmits<{ 'update:modelValue': [value: string | number | boolean]; change: [] }>()
const attrs = useAttrs()
const slots = useSlots()
const menuId = useId()
const trigger = ref<HTMLButtonElement | null>(null)
const menu = ref<HTMLElement | null>(null)
const open = ref(false)
const active = ref(0)
const menuStyle = ref<Record<string, string>>({})
type Option = { value: string | number | boolean; label: string; disabled: boolean }
function textOf(node: unknown): string {
  if (typeof node === 'string' || typeof node === 'number') return String(node)
  if (Array.isArray(node)) return node.map(textOf).join('')
  if (node && typeof node === 'object' && 'children' in node) return textOf((node as VNode).children)
  return ''
}
const options = computed<Option[]>(() => {
  const items: Option[] = []
  function visit(nodes: VNode[]) {
    for (const node of nodes) {
      if (node.type === 'option') {
        const label = textOf(node.children).trim()
        items.push({ label, value: (node.props?.value ?? label) as string | number | boolean, disabled: node.props?.disabled === '' || !!node.props?.disabled })
      } else if (Array.isArray(node.children)) visit(node.children as VNode[])
    }
  }
  visit(slots.default?.() ?? [])
  return items
})
const selected = computed(() => options.value.find(item => item.value === props.modelValue || String(item.value) === String(props.modelValue)))
function position() {
  if (!trigger.value) return
  const box = trigger.value.getBoundingClientRect()
  const height = Math.min(280, Math.max(48, options.value.length * 40 + 12))
  const above = window.innerHeight - box.bottom < height + 12 && box.top > window.innerHeight - box.bottom
  const width = Math.min(Math.max(box.width, 180), window.innerWidth - 16)
  menuStyle.value = { position: 'fixed', left: `${Math.max(8, Math.min(box.left, window.innerWidth - width - 8))}px`, width: `${width}px`, maxHeight: `${Math.max(80, Math.min(height, above ? box.top - 12 : window.innerHeight - box.bottom - 12))}px`, ...(above ? { bottom: `${window.innerHeight - box.top + 6}px` } : { top: `${box.bottom + 6}px` }) }
}
function show() {
  if (props.disabled) return
  active.value = Math.max(0, options.value.findIndex(item => item === selected.value))
  position(); open.value = true
  void nextTick(() => menu.value?.querySelector<HTMLElement>('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' }))
}
function choose(item: Option) {
  if (item.disabled) return
  const value = props.modelModifiers?.number ? Number(item.value) : item.value
  emit('update:modelValue', value); emit('change'); open.value = false
  trigger.value?.focus()
}
function keydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && open.value) { event.preventDefault(); open.value = false; return }
  if (event.key === 'Tab') { open.value = false; return }
  if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
    event.preventDefault()
    if (!open.value) { show(); return }
    const n = options.value.length
    if (!n) return
    let i = event.key === 'Home' ? 0 : event.key === 'End' ? n - 1 : (active.value + (event.key === 'ArrowDown' ? 1 : -1) + n) % n
    for (let count = 0; count < n && options.value[i]?.disabled; count++) i = (i + (event.key === 'ArrowUp' || event.key === 'End' ? -1 : 1) + n) % n
    active.value = i
    void nextTick(() => menu.value?.querySelector<HTMLElement>(`[data-index="${i}"]`)?.scrollIntoView({ block: 'nearest' }))
  } else if ((event.key === 'Enter' || event.key === ' ') && open.value) { event.preventDefault(); const item = options.value[active.value]; if (item) choose(item) }
}
function outside(event: MouseEvent) { if (open.value && !trigger.value?.contains(event.target as Node) && !menu.value?.contains(event.target as Node)) open.value = false }
function updatePosition() { if (open.value) position() }
onMounted(() => { document.addEventListener('mousedown', outside); window.addEventListener('resize', updatePosition); window.addEventListener('scroll', updatePosition, true) })
onBeforeUnmount(() => { document.removeEventListener('mousedown', outside); window.removeEventListener('resize', updatePosition); window.removeEventListener('scroll', updatePosition, true) })
</script>

<template>
  <div :class="['global-dropdown', attrs.class, { 'is-open': open, 'is-disabled': disabled }]">
    <button :id="attrs.id as string | undefined" ref="trigger" type="button" role="combobox" class="global-dropdown-trigger" :disabled="disabled" :aria-label="attrs['aria-label'] as string | undefined" aria-haspopup="listbox" :aria-controls="menuId" :aria-activedescendant="open && options.length ? `${menuId}-${active}` : undefined" :aria-expanded="open" @click="open ? open = false : show()" @keydown="keydown">
      <span class="global-dropdown-label">{{ selected?.label || 'Pilih opsi' }}</span><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 9 6 6 6-6" /></svg>
    </button>
    <Teleport to="body"><div v-if="open" :id="menuId" ref="menu" class="global-dropdown-menu" role="listbox" :aria-label="attrs['aria-label'] as string || 'Pilihan'" :style="menuStyle" @mousedown.prevent><button v-for="(item, index) in options" :id="`${menuId}-${index}`" :key="index" type="button" role="option" tabindex="-1" :data-index="index" :aria-selected="item === selected" :disabled="item.disabled" :class="['global-dropdown-option', { 'is-active': index === active, 'is-selected': item === selected }]" @mousemove="active = index" @click="choose(item)"><span>{{ item.label }}</span><svg v-if="item === selected" viewBox="0 0 24 24" aria-hidden="true"><path d="m5 12 4 4L19 6" /></svg></button><div v-if="!options.length" class="global-dropdown-empty">Tidak ada pilihan</div></div></Teleport>
  </div>
</template>
