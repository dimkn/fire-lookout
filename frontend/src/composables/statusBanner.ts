import { computed, ref } from 'vue'

export type BannerKind = 'success' | 'error'

export interface Banner {
  id: number
  kind: BannerKind
  message: string
}

// Module-scoped on purpose: anything in the app can announce an outcome without threading
// props or events through the tree. One queue, shown strictly one at a time.
const queue = ref<Banner[]>([])
let nextId = 0

/** The banner to show right now — the head of the queue, or null when there is none. */
export const currentBanner = computed<Banner | null>(() => queue.value[0] ?? null)

function notify(kind: BannerKind, message: string) {
  nextId += 1
  queue.value.push({ id: nextId, kind, message })
}

/** Announce a failed operation. */
export function notifyError(message: string) {
  notify('error', message)
}

/** Announce a completed operation. */
export function notifySuccess(message: string) {
  notify('success', message)
}

/** Drop the banner on screen; the next queued one takes its place. */
export function dismissBanner() {
  queue.value.shift()
}

/** Drop everything, shown and queued. */
export function clearBanners() {
  queue.value = []
}
