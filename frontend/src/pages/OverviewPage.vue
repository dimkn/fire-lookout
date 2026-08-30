<script setup lang="ts">
import { onMounted, ref } from 'vue'

import type { StatusItem, SystemOverview } from '@/api/status'
import { fetchFeedItems, fetchOverview } from '@/api/status'
import SystemList from '@/components/SystemList/SystemList.vue'
import PageToolbar from '@/components/PageToolbar/PageToolbar.vue'

const systems = ref<SystemOverview[]>([])
const itemsByFeed = ref<Record<number, StatusItem[]>>({})

onMounted(async () => {
  try {
    systems.value = await fetchOverview()
  } catch (error) {
    // No loading or error state in this slice: an empty area is the honest fallback.
    console.error(error)
  }
})

// A card asks for its incidents the first time it opens; we keep them, so re-opening
// costs nothing. Nothing invalidates the cache yet — Refresh is still inert.
async function loadItems(feedId: number) {
  if (feedId in itemsByFeed.value) {
    return
  }
  try {
    itemsByFeed.value[feedId] = await fetchFeedItems(feedId)
  } catch (error) {
    console.error(error)
  }
}

function refresh() {
  // No-op for now: re-polling feeds on demand is its own slice.
}

function addIntegration() {
  // No-op for now: subscribing to a new feed is its own slice.
}
</script>

<template>
  <main class="overview">
    <PageToolbar @refresh="refresh" @add="addIntegration" />

    <div class="overview__systems">
      <SystemList :systems="systems" :items-by-feed="itemsByFeed" @expand="loadItems" />
    </div>
  </main>
</template>

<style scoped>
.overview__systems {
  padding-top: 0.5rem;
}
</style>
