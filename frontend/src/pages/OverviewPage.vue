<script setup lang="ts">
import { onMounted, ref } from 'vue'

import type { StatusItem, SystemOverview } from '@/api/status'
import { fetchFeedItems, fetchOverview } from '@/api/status'
import SystemList from '@/components/SystemList/SystemList.vue'
import PageToolbar from '@/components/PageToolbar/PageToolbar.vue'
import { notifyError, notifySuccess } from '@/composables/statusBanner'
import { delay, MIN_LOADING_MS } from '@/utils/timing'

const systems = ref<SystemOverview[]>([])
const itemsByFeed = ref<Record<number, StatusItem[]>>({})
const refreshing = ref(false)

onMounted(async () => {
  try {
    // Assigning only after the request resolves is what leaves the screen untouched when
    // a load fails.
    systems.value = await fetchOverview()
  } catch (error) {
    // Without this the empty area would be indistinguishable from "nothing saved yet".
    console.error(error)
    notifyError('Could not load the status overview.')
  }
})

// A card asks for its incidents the first time it opens; we keep them, so re-opening costs
// nothing. Refresh deliberately does not invalidate them — it re-reads the overview only.
async function loadItems(feedId: number) {
  if (feedId in itemsByFeed.value) {
    return
  }
  try {
    itemsByFeed.value[feedId] = await fetchFeedItems(feedId)
  } catch (error) {
    // An empty card body would otherwise read as "this system has no incidents".
    console.error(error)
    const name = systems.value.find((system) => system.feed.id === feedId)?.feed.title
    notifyError(`Could not load incidents for ${name ?? 'this system'}.`)
  }
}

// Re-reads the overview only. Already-fetched incidents are kept: asking each open card's
// feed for items again would mean more requests than the button promises. Re-polling the
// remote feeds themselves is still a separate slice.
async function refresh() {
  if (refreshing.value) {
    return
  }
  refreshing.value = true

  // A floor, not a timeout: a slow request still owns the loader for as long as it takes.
  // Both the fresh data and the idle button land after it, so no frame shows new rows
  // underneath a spinning loader.
  const minimumElapsed = delay(MIN_LOADING_MS)
  try {
    const fresh = await fetchOverview()
    await minimumElapsed
    systems.value = fresh
    notifySuccess('Status updated.')
  } catch (error) {
    // Leave the current data on screen — a failed refresh must not blank the page.
    await minimumElapsed
    console.error(error)
    notifyError('Could not refresh. Showing the last known status.')
  } finally {
    refreshing.value = false
  }
}

function addIntegration() {
  // No-op for now: subscribing to a new feed is its own slice.
}
</script>

<template>
  <main class="overview">
    <PageToolbar :refreshing="refreshing" @refresh="refresh" @add="addIntegration" />

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
