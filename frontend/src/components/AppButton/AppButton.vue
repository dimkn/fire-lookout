<script setup lang="ts">
import DotsLoader from '@/components/DotsLoader/DotsLoader.vue'

const props = withDefaults(defineProps<{ label: string; loading?: boolean }>(), {
  loading: false,
})

const emit = defineEmits<{ click: [] }>()

function onClick() {
  // `disabled` already blocks this; the guard keeps the contract explicit for callers
  // that drive the component programmatically.
  if (props.loading) {
    return
  }
  emit('click')
}
</script>

<template>
  <button
    class="app-button"
    type="button"
    :disabled="loading"
    :aria-busy="loading"
    @click="onClick"
  >
    <!-- The label stays in the DOM while loading: it reserves the button's width so the
         loader cannot resize it, and (being merely transparent, not hidden) it keeps the
         button's accessible name. -->
    <span class="app-button__label" :class="{ 'app-button__label--loading': loading }">{{
      label
    }}</span>
    <DotsLoader v-if="loading" class="app-button__loader" />
  </button>
</template>

<style scoped>
.app-button {
  position: relative;
  padding: 0.4rem 0.9rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
  cursor: pointer;
}

/* Resting appearance is identical while loading — only the hover cue drops away, since
   there is nothing to activate. */
.app-button:hover:not(:disabled) {
  background: var(--surface-hover);
}

.app-button:disabled {
  cursor: default;
}

.app-button:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.app-button__label--loading {
  opacity: 0;
}

.app-button__loader {
  position: absolute;
  inset: 0;
}
</style>
