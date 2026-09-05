<script setup lang="ts">
// Three dots hopping in turn: each dot hops for HOP_MS, and the next one starts as the
// previous one lands, so the cycle is DOT_COUNT hops long and only one dot is ever in the
// air. Delay and duration are inline (rather than nth-child CSS) so the sequencing is
// explicit and testable.
const DOT_COUNT = 3
const HOP_MS = 400

const cycleMs = DOT_COUNT * HOP_MS

const dots = Array.from({ length: DOT_COUNT }, (_, index) => ({
  index,
  style: {
    animationDelay: `${index * HOP_MS}ms`,
    animationDuration: `${cycleMs}ms`,
  },
}))
</script>

<template>
  <span class="dots-loader" aria-hidden="true">
    <span v-for="dot in dots" :key="dot.index" class="dots-loader__dot" :style="dot.style" />
  </span>
</template>

<style scoped>
.dots-loader {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.25rem;
}

.dots-loader__dot {
  width: 0.3rem;
  height: 0.3rem;
  border-radius: 50%;
  /* Inherit the host's ink, so a loader looks native wherever it is dropped in. */
  background: currentColor;
  animation-name: dots-loader-hop;
  animation-timing-function: ease-in-out;
  animation-iteration-count: infinite;
}

/* The hop occupies the first third of the cycle — one of DOT_COUNT slots — and the dot
   rests for the other two while its neighbours take their turn. */
@keyframes dots-loader-hop {
  0%,
  33.34%,
  100% {
    transform: translateY(0);
  }

  16.67% {
    transform: translateY(-0.3rem);
  }
}

@media (prefers-reduced-motion: reduce) {
  .dots-loader__dot {
    animation: none;
  }
}
</style>
