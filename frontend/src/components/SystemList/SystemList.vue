<script setup lang="ts">
import type { StatusItem, SystemOverview } from '@/api/status'
import SystemCard from '@/components/SystemCard/SystemCard.vue'

const props = withDefaults(
  defineProps<{
    systems: SystemOverview[]
    itemsByFeed?: Record<number, StatusItem[]>
  }>(),
  { itemsByFeed: () => ({}) },
)

const emit = defineEmits<{ expand: [feedId: number] }>()

// No ordering of our own: the API's order is the order (and the list is deliberately
// unpaginated for now).
function itemsFor(feedId: number): StatusItem[] {
  return props.itemsByFeed[feedId] ?? []
}
</script>

<template>
  <div class="system-list">
    <SystemCard
      v-for="system in systems"
      :key="system.feed.id"
      :system="system"
      :items="itemsFor(system.feed.id)"
      @expand="emit('expand', $event)"
    />
  </div>
</template>

<style scoped>
.system-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
</style>
