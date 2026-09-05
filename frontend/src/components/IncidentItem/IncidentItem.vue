<script setup lang="ts">
import { computed } from 'vue'

import type { Status, StatusItem } from '@/api/status'
import { formatTimestamp } from '@/utils/datetime'

const props = defineProps<{ item: StatusItem }>()

// Wording only: which status an incident has is decided by the backend.
const STATUS_LABELS: Record<Status, string> = {
  investigating: 'Investigating',
  identified: 'Identified',
  monitoring: 'Monitoring',
  resolved: 'Resolved',
  maintenance: 'Maintenance',
  unknown: 'Unknown',
}

const statusLabel = computed(() => STATUS_LABELS[props.item.status] ?? STATUS_LABELS.unknown)

// Providers may ship no timestamps at all; then the only thing we honestly know is when
// we first fetched the entry, so say exactly that.
const dated = computed(() => Boolean(props.item.published_at))
const timestamp = computed(() => props.item.published_at ?? props.item.fetched_at)
</script>

<template>
  <li class="incident">
    <div class="incident__head">
      <span class="incident__status">{{ statusLabel }}</span>
      <a
        v-if="item.link"
        class="incident__title"
        :href="item.link"
        target="_blank"
        rel="noopener noreferrer"
        >{{ item.title }}</a
      >
      <span v-else class="incident__title">{{ item.title }}</span>
    </div>

    <p class="incident__meta">
      <span v-if="!dated">First seen </span>
      <time :datetime="timestamp">{{ formatTimestamp(timestamp) }}</time>
    </p>

    <!-- Plaintext by design: the raw feed body is remote HTML and is never injected. -->
    <p v-if="item.content_text" class="incident__body">{{ item.content_text }}</p>
  </li>
</template>

<style scoped>
.incident {
  padding: 0.75rem 0;
  border-top: 1px solid var(--border-subtle);
  list-style: none;
}

.incident__head {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.5rem;
}

.incident__status {
  padding: 0.1rem 0.45rem;
  border: 1px solid var(--border);
  border-radius: 999px;
  color: var(--text-muted);
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  white-space: nowrap;
}

.incident__title {
  color: var(--text);
  font-weight: 600;
}

a.incident__title:hover {
  text-decoration: underline;
}

.incident__meta {
  margin: 0.25rem 0 0;
  color: var(--text-muted);
  font-size: 0.8rem;
}

.incident__body {
  margin: 0.4rem 0 0;
  color: var(--text-soft);
  line-height: 1.5;
}
</style>
