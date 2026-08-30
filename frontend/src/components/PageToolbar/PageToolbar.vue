<script setup lang="ts">
import AppButton from '@/components/AppButton/AppButton.vue'

const emit = defineEmits<{ refresh: []; add: [] }>()
</script>

<template>
  <div class="toolbar">
    <AppButton label="Refresh" @click="emit('refresh')" />
    <AppButton label="Add New Status Integration" @click="emit('add')" />
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.75rem 0;
}
</style>
