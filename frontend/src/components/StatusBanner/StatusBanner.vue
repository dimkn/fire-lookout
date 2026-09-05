<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'

import type { BannerKind } from '@/composables/statusBanner'
import { truncate } from '@/utils/text'
import { BANNER_TIMEOUT_MS } from '@/utils/timing'

// The banner is a fixed-width box; capping the message keeps it to three lines at most
// (see the width note in the styles below).
const MAX_MESSAGE_LENGTH = 100

const props = defineProps<{ kind: BannerKind; message: string }>()

const emit = defineEmits<{ close: [] }>()

const text = computed(() => truncate(props.message, MAX_MESSAGE_LENGTH))

// Errors interrupt; successes wait their turn in the screen reader's queue.
const role = computed(() => (props.kind === 'error' ? 'alert' : 'status'))

let timer: ReturnType<typeof setTimeout> | undefined

onMounted(() => {
  timer = setTimeout(() => emit('close'), BANNER_TIMEOUT_MS)
})

onBeforeUnmount(() => {
  clearTimeout(timer)
})
</script>

<template>
  <div class="status-banner" :class="`status-banner--${kind}`" :role="role">
    <p class="status-banner__message">{{ text }}</p>
    <button class="status-banner__close" type="button" aria-label="Close" @click="emit('close')">
      <span aria-hidden="true">&times;</span>
    </button>
  </div>
</template>

<style scoped>
/* Width is derived, not guessed. Three lines must hold the 100-character cap, so a line
   needs ceil(100 / 3) = 34 characters. Measured in the app's own 14px -apple-system:
   lowercase prose averages 6.4px per character, ALL CAPS 8.2px. Sizing for the denser of
   the two, 34 x 8.2px = 279px of text, plus 14px padding either side and 24px reserved for
   the close button, gives 331px — rounded to 21rem (336px). The box never grows: long
   messages wrap inside it.

   Uniform widest-glyph strings (100 x "W", 13.4px each) would still spill to a fourth
   line; guaranteeing those would need ~508px and waste that space on every real message. */
.status-banner {
  position: fixed;
  right: 1rem;
  bottom: 1rem;
  z-index: var(--z-status-banner);
  width: 21rem;
  padding: 0.75rem 0.875rem;
  border-radius: 6px;
  box-shadow: 0 4px 12px rgb(31 35 40 / 25%);
  color: #fff;
}

/* The two modes differ only in colour, taken from the traffic-light palette and darkened
   so white text clears WCAG AA on both. */
.status-banner--success {
  background: color-mix(in srgb, var(--indicator-operational) 75%, #000);
}

.status-banner--error {
  background: color-mix(in srgb, var(--indicator-outage) 75%, #000);
}

.status-banner__message {
  margin: 0;
  /* Keeps every line clear of the close button, whatever the message length. */
  padding-right: 1.5rem;
  /* An unbroken 100-character run still wraps rather than widening the box. */
  overflow-wrap: anywhere;
}

/* Offset from the top and right edges so it reads as sitting at the end of the first
   line, regardless of how many lines the message takes. */
.status-banner__close {
  position: absolute;
  top: 0.5rem;
  right: 0.5rem;
  width: 1.25rem;
  height: 1.25rem;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: none;
  color: inherit;
  font-size: 1rem;
  line-height: 1;
  cursor: pointer;
}

.status-banner__close:hover {
  background: rgb(255 255 255 / 20%);
}

.status-banner__close:focus-visible {
  outline: 2px solid #fff;
  outline-offset: 1px;
}
</style>
