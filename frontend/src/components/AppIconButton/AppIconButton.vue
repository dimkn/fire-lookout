<script setup lang="ts">
// An icon-only button. The label is never drawn — it is the accessible name and the tooltip,
// which is the whole reason a caller must supply one.
//
// pressed is for toggles: leave it undefined and no aria-pressed attribute is emitted at all,
// so a plain action button is not mistaken for a toggle that happens to be off.
//
// The explicit `pressed: undefined` default is load-bearing: Vue otherwise casts an absent
// Boolean prop to false, which would put aria-pressed="false" on every plain action button.
const props = withDefaults(
  defineProps<{ label: string; pressed?: boolean; disabled?: boolean }>(),
  { pressed: undefined, disabled: false },
)

const emit = defineEmits<{ click: [] }>()

function onClick() {
  if (props.disabled) {
    return
  }
  emit('click')
}
</script>

<template>
  <button
    class="app-icon-button"
    type="button"
    :aria-label="label"
    :title="label"
    :aria-pressed="pressed"
    :disabled="disabled"
    @click="onClick"
  >
    <slot />
  </button>
</template>

<style scoped>
.app-icon-button {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 1.75rem;
  height: 1.75rem;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 6px;
  background: none;
  color: var(--text-muted);
  cursor: pointer;
}

.app-icon-button:hover {
  border-color: var(--border);
  background: var(--surface-hover);
  color: var(--text);
}

.app-icon-button:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}

/* Deliberately no styling keyed on aria-pressed: what "off" looks like belongs to whatever
   the toggle controls — SystemCard dims its entire row — and a second dim here would stack
   into near-invisibility. */
</style>
