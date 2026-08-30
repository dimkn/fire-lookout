<script setup lang="ts">
defineProps<{ label: string }>()

const emit = defineEmits<{ click: [] }>()
</script>

<template>
  <button class="app-button" type="button" @click="emit('click')">{{ label }}</button>
</template>

<style scoped>
.app-button {
  padding: 0.4rem 0.9rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
  cursor: pointer;
}

.app-button:hover {
  background: var(--surface-hover);
}

.app-button:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
</style>
