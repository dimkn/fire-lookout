<script setup lang="ts">
import { useId } from 'vue'

export interface SelectOption {
  label: string
  value: number
}

defineProps<{ label: string; modelValue: number; options: SelectOption[] }>()

const emit = defineEmits<{ 'update:modelValue': [number] }>()

const selectId = useId()

// A <select> always yields strings; converting here means no caller has to remember to.
function onChange(event: Event) {
  emit('update:modelValue', Number((event.target as HTMLSelectElement).value))
}
</script>

<template>
  <label class="app-select" :for="selectId">
    <span class="app-select__label">{{ label }}</span>
    <select :id="selectId" class="app-select__input" :value="modelValue" @change="onChange">
      <option v-for="option in options" :key="option.value" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  </label>
</template>

<style scoped>
.app-select {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.app-select__label {
  color: var(--text-soft);
  font-weight: 600;
}

.app-select__input {
  padding: 0.4rem 0.5rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
}

.app-select__input:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}
</style>
