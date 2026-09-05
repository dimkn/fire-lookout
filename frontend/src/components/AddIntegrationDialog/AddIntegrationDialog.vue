<script setup lang="ts">
import { computed, ref } from 'vue'

import type { SubscribeFeedRequest } from '@/api/status'
import AppButton from '@/components/AppButton/AppButton.vue'
import AppModal from '@/components/AppModal/AppModal.vue'
import AppSelect from '@/components/AppSelect/AppSelect.vue'
import type { SelectOption } from '@/components/AppSelect/AppSelect.vue'
import AppTextField from '@/components/AppTextField/AppTextField.vue'

// Values are seconds — the unit the request, the database and the poller all use. The 30s
// floor matches the contract's minimum.
const INTERVAL_OPTIONS: SelectOption[] = [
  { label: '30 seconds', value: 30 },
  { label: '1 minute', value: 60 },
  { label: '5 minutes', value: 300 },
  { label: '10 minutes', value: 600 },
  { label: '30 minutes', value: 1800 },
]

const DEFAULT_INTERVAL_SEC = 300

const props = withDefaults(defineProps<{ saving?: boolean }>(), { saving: false })

const emit = defineEmits<{ close: []; submit: [SubscribeFeedRequest] }>()

const name = ref('')
const url = ref('')
const intervalSec = ref(DEFAULT_INTERVAL_SEC)

// Validated on every change: both text fields must hold something once trimmed. The cadence
// always holds one of the offered values, so there is nothing to validate there.
const canSave = computed(() => name.value.trim() !== '' && url.value.trim() !== '')

function submit() {
  if (!canSave.value || props.saving) {
    return
  }
  emit('submit', {
    title: name.value.trim(),
    url: url.value.trim(),
    refresh_interval_sec: intervalSec.value,
  })
}
</script>

<template>
  <AppModal title="Add New Status Integration" @close="emit('close')">
    <!-- Enter submits, which is why this is a real form. -->
    <form class="add-integration" @submit.prevent="submit">
      <AppTextField v-model="name" label="Name" />
      <AppTextField v-model="url" label="RSS link" type="url" />
      <AppSelect v-model="intervalSec" label="Check every" :options="INTERVAL_OPTIONS" />
    </form>

    <template #actions>
      <AppButton label="Close" @click="emit('close')" />
      <AppButton label="Save" :loading="saving" :disabled="!canSave" @click="submit" />
    </template>
  </AppModal>
</template>

<style scoped>
.add-integration {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
</style>
