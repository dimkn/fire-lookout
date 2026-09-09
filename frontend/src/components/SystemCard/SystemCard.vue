<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import type { StatusItem, SystemOverview } from '@/api/status'
import AppIconButton from '@/components/AppIconButton/AppIconButton.vue'
import IncidentItem from '@/components/IncidentItem/IncidentItem.vue'
import StatusIndicator from '@/components/StatusIndicator/StatusIndicator.vue'
import { formatTimestamp } from '@/utils/datetime'

const props = withDefaults(
  defineProps<{ system: SystemOverview; items?: StatusItem[]; busy?: boolean }>(),
  { items: () => [], busy: false },
)

const emit = defineEmits<{
  expand: [feedId: number]
  setEnabled: [payload: { feedId: number; enabled: boolean }]
}>()

// Expansion is this card's own state, so opening one card leaves the others untouched.
const expanded = ref(false)
const asked = ref(false)

const panelId = computed(() => `system-${props.system.feed.id}-detail`)
const paused = computed(() => !props.system.feed.enabled)

function toggle() {
  // A paused system has nothing current to show, so it does not open.
  if (paused.value) {
    return
  }
  expanded.value = !expanded.value

  // Ask the page for this system's incidents the first time only — it keeps them.
  if (expanded.value && !asked.value) {
    asked.value = true
    emit('expand', props.system.feed.id)
  }
}

function setEnabled() {
  emit('setEnabled', { feedId: props.system.feed.id, enabled: paused.value })
}

// Pausing a system that is open closes it: the detail describes a state we have stopped
// tracking, so leaving it on screen would keep asserting something we no longer check.
watch(paused, (isPaused) => {
  if (isPaused) {
    expanded.value = false
  }
})
</script>

<template>
  <article class="system-card">
    <!-- The header is a plain element with a button inside it, not one big button: nesting
         interactive controls inside a button is invalid, and a click on Delete must never
         also expand the card. -->
    <div class="system-card__header" :class="{ 'system-card__header--paused': paused }">
      <button
        class="system-card__row"
        type="button"
        :aria-expanded="expanded"
        :aria-controls="panelId"
        :disabled="paused"
        @click="toggle"
      >
        <StatusIndicator :indicator="system.indicator" />
        <span class="system-card__name">{{ system.feed.title }}</span>
        <span v-if="system.last_updated_at" class="system-card__updated">
          <time :datetime="system.last_updated_at">{{
            formatTimestamp(system.last_updated_at)
          }}</time>
        </span>
      </button>

      <!-- Edit and Delete are still visual only; the switch is wired up. -->
      <div class="system-card__actions">
        <AppIconButton label="Edit integration">
          <svg
            viewBox="0 0 16 16"
            width="15"
            height="15"
            fill="none"
            stroke="currentColor"
            stroke-width="1.4"
            aria-hidden="true"
          >
            <path d="M2.5 13.5h3l8-8-3-3-8 8v3z" />
            <path d="M10 3l3 3" />
          </svg>
        </AppIconButton>

        <AppIconButton label="Delete integration">
          <svg
            viewBox="0 0 16 16"
            width="15"
            height="15"
            fill="none"
            stroke="currentColor"
            stroke-width="1.4"
            aria-hidden="true"
          >
            <path d="M3 4.5h10" />
            <path d="M6.25 4.5V3h3.5v1.5" />
            <path d="M4.5 4.5l.6 9h5.8l.6-9" />
            <path d="M6.75 7v4M9.25 7v4" />
          </svg>
        </AppIconButton>

        <AppIconButton
          :label="system.feed.enabled ? 'Turn polling off' : 'Turn polling on'"
          :pressed="system.feed.enabled"
          :disabled="busy"
          @click="setEnabled"
        >
          <svg
            viewBox="0 0 16 16"
            width="15"
            height="15"
            fill="none"
            stroke="currentColor"
            stroke-width="1.4"
            aria-hidden="true"
          >
            <path d="M8 2.5v4.5" />
            <path d="M4.9 4.6a4.5 4.5 0 1 0 6.2 0" />
          </svg>
        </AppIconButton>
      </div>
    </div>

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

/* The header is one line: the toggle takes the slack, the actions keep their size. */
.system-card__header {
  display: flex;
  align-items: center;
  border-radius: 8px;
}

.system-card__header:hover {
  background: var(--surface-hover);
}

/* A paused system is dimmed as a whole row — indicator, name, timestamp and actions — so it
   reads as "not being watched" at a glance rather than only via the toggle icon.
   0.7 rather than something heavier on purpose: it is visibly muted while keeping the name
   above the AA contrast threshold, which a paused row still needs to be readable. */
.system-card__header--paused {
  opacity: 0.7;
}

.system-card__row {
  display: flex;
  flex: 1 1 auto;
  align-items: center;
  gap: 0.75rem;
  min-width: 0;
  padding: 0.7rem 0.9rem;
  border: 0;
  border-radius: 8px;
  background: none;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.system-card__row:disabled {
  cursor: default;
}

.system-card__actions {
  display: flex;
  flex: none;
  align-items: center;
  gap: 0.15rem;
  padding-right: 0.6rem;
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
