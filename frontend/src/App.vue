<script setup lang="ts">
import OverviewPage from '@/pages/OverviewPage.vue'

// Injected from the root VERSION file at build time (AGENTS.md §7). Carried on the root
// element so the running build is identifiable without any chrome on the page.
const version = __APP_VERSION__
</script>

<template>
  <div class="app" :data-app-version="version">
    <OverviewPage />
  </div>
</template>

<style scoped>
/* The working area: 95% of the viewport, never narrower than 480px (below that the page
   scrolls horizontally rather than cramping the rows). */
.app {
  width: 95%;
  min-width: 480px;
  margin: 0 auto;
  padding: 1rem 0 3rem;
}
</style>
