<script setup lang="ts">
import { useId } from 'vue'

withDefaults(defineProps<{ label: string; modelValue: string; type?: string }>(), {
  type: 'text',
})

const emit = defineEmits<{ 'update:modelValue': [string] }>()

// Ties the label to its input without the caller having to invent ids.
const inputId = useId()

function onInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLInputElement).value)
}
</script>

<template>
  <label class="app-text-field" :for="inputId">
    <span class="app-text-field__label">{{ label }}</span>
    <input
      :id="inputId"
      class="app-text-field__input"
      :type="type"
      :value="modelValue"
      autocomplete="off"
      spellcheck="false"
      @input="onInput"
    />
  </label>
</template>

<style scoped>
.app-text-field {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.app-text-field__label {
  color: var(--text-soft);
  font-weight: 600;
}

.app-text-field__input {
  padding: 0.4rem 0.5rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
}

.app-text-field__input:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}
</style>
