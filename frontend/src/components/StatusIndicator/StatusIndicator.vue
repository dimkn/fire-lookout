<script setup lang="ts">
import { computed } from 'vue'

import type { Indicator } from '@/api/status'

const props = defineProps<{ indicator: Indicator }>()

// Wording only — which indicator a system has is decided by the backend.
const LABELS: Record<Indicator, string> = {
  operational: 'Operational',
  degraded: 'Degraded',
  outage: 'Outage',
  unknown: 'Unknown',
}

const label = computed(() => LABELS[props.indicator] ?? LABELS.unknown)
</script>

<template>
  <span
    class="status-indicator"
    :class="`status-indicator--${indicator}`"
    role="img"
    :aria-label="label"
    :title="label"
  />
</template>

<style scoped>
.status-indicator {
  display: inline-block;
  flex: none;
  width: 0.75rem;
  height: 0.75rem;
  border-radius: 50%;
  background: var(--indicator-color, var(--indicator-unknown));
  box-shadow: 0 0 0 3px
    color-mix(in srgb, var(--indicator-color, var(--indicator-unknown)) 22%, transparent);
}

.status-indicator--operational {
  --indicator-color: var(--indicator-operational);
}

.status-indicator--degraded {
  --indicator-color: var(--indicator-degraded);
}

.status-indicator--outage {
  --indicator-color: var(--indicator-outage);
}

.status-indicator--unknown {
  --indicator-color: var(--indicator-unknown);
}
</style>
