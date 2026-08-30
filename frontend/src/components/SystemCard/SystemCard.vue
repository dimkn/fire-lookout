<script setup lang="ts">
import { computed, ref } from 'vue'

import type { StatusItem, SystemOverview } from '@/api/status'
import IncidentItem from '@/components/IncidentItem/IncidentItem.vue'
import StatusIndicator from '@/components/StatusIndicator/StatusIndicator.vue'
import { formatTimestamp } from '@/utils/datetime'

const props = withDefaults(defineProps<{ system: SystemOverview; items?: StatusItem[] }>(), {
  items: () => [],
})

const emit = defineEmits<{ expand: [feedId: number] }>()

// Expansion is this card's own state, so opening one card leaves the others untouched.
const expanded = ref(false)
const asked = ref(false)

const panelId = computed(() => `system-${props.system.feed.id}-detail`)

function toggle() {
  expanded.value = !expanded.value

  // Ask the page for this system's incidents the first time only — it keeps them.
  if (expanded.value && !asked.value) {
    asked.value = true
    emit('expand', props.system.feed.id)
  }
}
</script>

<template>
  <article class="system-card">
    <button
      class="system-card__row"
      type="button"
      :aria-expanded="expanded"
      :aria-controls="panelId"
      @click="toggle"
    >
      <StatusIndicator :indicator="system.indicator" />
      <span class="system-card__name">{{ system.feed.title }}</span>
      <span v-if="system.last_updated_at" class="system-card__updated">
        <time :datetime="system.last_updated_at">{{
          formatTimestamp(system.last_updated_at)
        }}</time>
      </span>
      <span class="system-card__chevron" aria-hidden="true">{{ expanded ? '▾' : '▸' }}</span>
    </button>

    <section v-if="expanded" :id="panelId" class="system-card__detail">
      <dl class="system-card__facts">
        <dt>Feed</dt>
        <dd>
          <a :href="system.feed.url" target="_blank" rel="noopener noreferrer">{{
            system.feed.url
          }}</a>
        </dd>

        <dt>Last checked</dt>
        <dd>
          <time v-if="system.feed.last_fetched_at" :datetime="system.feed.last_fetched_at">{{
            formatTimestamp(system.feed.last_fetched_at)
          }}</time>
          <span v-else>Never</span>
        </dd>

        <dt>Last successful check</dt>
        <dd>
          <time v-if="system.feed.last_success_at" :datetime="system.feed.last_success_at">{{
            formatTimestamp(system.feed.last_success_at)
          }}</time>
          <span v-else>Never</span>
        </dd>
      </dl>

      <p v-if="system.feed.last_error" class="system-card__error">
        Last check failed: {{ system.feed.last_error }}
      </p>

      <ul v-if="items.length" class="system-card__incidents">
        <IncidentItem v-for="item in items" :key="item.id" :item="item" />
      </ul>
    </section>
  </article>
</template>

<style scoped>
.system-card {
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
}

.system-card__row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  width: 100%;
  padding: 0.7rem 0.9rem;
  border: 0;
  border-radius: 8px;
  background: none;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.system-card__row:hover {
  background: var(--surface-hover);
}

.system-card__row:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}

/* One line per system: the name takes the slack and truncates rather than wrapping. */
.system-card__name {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  font-weight: 600;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.system-card__updated {
  flex: none;
  color: var(--text-muted);
  font-size: 0.8rem;
  white-space: nowrap;
}

.system-card__chevron {
  flex: none;
  color: var(--text-muted);
}

.system-card__detail {
  padding: 0 0.9rem 0.9rem;
}

.system-card__facts {
  display: grid;
  grid-template-columns: minmax(9rem, max-content) 1fr;
  gap: 0.2rem 1rem;
  margin: 0;
  padding-top: 0.5rem;
  border-top: 1px solid var(--border-subtle);
  font-size: 0.85rem;
}

.system-card__facts dt {
  color: var(--text-muted);
}

.system-card__facts dd {
  margin: 0;
  overflow-wrap: anywhere;
}

.system-card__error {
  margin: 0.6rem 0 0;
  color: var(--danger);
  font-size: 0.85rem;
}

.system-card__incidents {
  margin: 0.6rem 0 0;
  padding: 0;
}
</style>
