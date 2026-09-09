<script setup lang="ts">
import { onMounted, ref } from 'vue'

import type { StatusItem, SubscribeFeedRequest, SystemOverview } from '@/api/status'
import {
  fetchFeedItems,
  fetchOverview,
  RequestError,
  subscribeFeed,
  updateFeed,
} from '@/api/status'
import AddIntegrationDialog from '@/components/AddIntegrationDialog/AddIntegrationDialog.vue'
import SystemList from '@/components/SystemList/SystemList.vue'
import PageToolbar from '@/components/PageToolbar/PageToolbar.vue'
import { notifyError, notifySuccess } from '@/composables/statusBanner'
import { delay, MIN_LOADING_MS } from '@/utils/timing'

const systems = ref<SystemOverview[]>([])
const itemsByFeed = ref<Record<number, StatusItem[]>>({})
const refreshing = ref(false)
const addOpen = ref(false)
const saving = ref(false)
const togglingFeedIds = ref<number[]>([])

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
  addOpen.value = true
}

// Unmounting the dialog is what clears it, so reopening always offers a blank form.
function closeAdd() {
  addOpen.value = false
}

async function saveIntegration(input: SubscribeFeedRequest) {
  if (saving.value) {
    return
  }
  saving.value = true

  const minimumElapsed = delay(MIN_LOADING_MS)
  try {
    await subscribeFeed(input)
    await minimumElapsed
    addOpen.value = false
    notifySuccess(`${input.title} was added.`)
    await reloadAfterAdd()
  } catch (error) {
    // The dialog stays exactly as it was, so the user can fix the value and retry. The
    // backend's own message is the useful one — it says which rule was broken.
    await minimumElapsed
    console.error(error)
    const message =
      error instanceof RequestError && error.serverMessage
        ? error.serverMessage
        : 'Could not add the integration.'
    notifyError(message)
  } finally {
    saving.value = false
  }
}

// Flipping the pause switch. The row's new colour comes from the backend — a paused system
// reports "unknown" — so the list is re-read rather than patched up here.
async function setEnabled({ feedId, enabled }: { feedId: number; enabled: boolean }) {
  if (togglingFeedIds.value.includes(feedId)) {
    return
  }
  togglingFeedIds.value = [...togglingFeedIds.value, feedId]

  try {
    await updateFeed(feedId, { enabled })
    systems.value = await fetchOverview()
  } catch (error) {
    // No success banner: the row visibly changes colour, which says it better than a message
    // would. A failure has nothing to show, so it has to be announced.
    console.error(error)
    const message =
      error instanceof RequestError && error.serverMessage
        ? error.serverMessage
        : `Could not ${enabled ? 'resume' : 'pause'} that integration.`
    notifyError(message)
  } finally {
    togglingFeedIds.value = togglingFeedIds.value.filter((id) => id !== feedId)
  }
}

// The new system is stored but the dashboard renders the overview read model, so re-read it
// rather than deriving a row here — which light a system shows is the backend's decision.
async function reloadAfterAdd() {
  try {
    systems.value = await fetchOverview()
  } catch (error) {
    console.error(error)
    notifyError('Added, but the list could not be reloaded. Press Refresh.')
  }
}
</script>

<template>
  <main class="overview">
    <PageToolbar :refreshing="refreshing" @refresh="refresh" @add="addIntegration" />

    <div class="overview__systems">
      <SystemList
        :systems="systems"
        :items-by-feed="itemsByFeed"
        :busy-feed-ids="togglingFeedIds"
        @expand="loadItems"
        @set-enabled="setEnabled"
      />
    </div>

    <AddIntegrationDialog
      v-if="addOpen"
      :saving="saving"
      @close="closeAdd"
      @submit="saveIntegration"
    />
  </main>
</template>

<style scoped>
.overview__systems {
  padding-top: 0.5rem;
}
</style>
