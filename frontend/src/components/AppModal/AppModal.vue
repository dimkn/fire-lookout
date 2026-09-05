<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'

defineProps<{ title: string }>()

const emit = defineEmits<{ close: [] }>()

// Escape is the keyboard equivalent of the Close button. Clicking the backdrop
// deliberately does nothing: it is too easy to hit by accident and lose a half-typed form.
function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    emit('close')
  }
}

onMounted(() => document.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="app-modal">
    <div class="app-modal__backdrop" />
    <div class="app-modal__panel" role="dialog" aria-modal="true" :aria-label="title">
      <h2 class="app-modal__title">{{ title }}</h2>
      <div class="app-modal__body">
        <slot />
      </div>
      <div class="app-modal__actions">
        <slot name="actions" />
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Above the page, below the status banner — a failure while the dialog is open has to be
   readable. */
.app-modal {
  position: fixed;
  inset: 0;
  z-index: var(--z-modal);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
}

.app-modal__backdrop {
  position: absolute;
  inset: 0;
  background: rgb(31 35 40 / 45%);
}

.app-modal__panel {
  position: relative;
  width: min(28rem, 100%);
  padding: 1rem 1.25rem 1.25rem;
  border-radius: 8px;
  background: var(--surface);
  box-shadow: 0 8px 32px rgb(31 35 40 / 30%);
}

.app-modal__title {
  margin: 0 0 0.75rem;
  font-size: 1rem;
}

.app-modal__actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-top: 1.25rem;
}
</style>
