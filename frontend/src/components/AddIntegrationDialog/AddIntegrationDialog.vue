<script setup lang="ts">
import { computed, ref } from 'vue'

import AppButton from '@/components/AppButton/AppButton.vue'
import AppModal from '@/components/AppModal/AppModal.vue'
import AppTextField from '@/components/AppTextField/AppTextField.vue'

const props = withDefaults(defineProps<{ saving?: boolean }>(), { saving: false })

const emit = defineEmits<{ close: []; submit: [{ title: string; url: string }] }>()

const name = ref('')
const url = ref('')

// Validated on every change: both fields must hold something once trimmed. Trimming here
// matches what the backend does, so the button never invites a request bound to fail.
const canSave = computed(() => name.value.trim() !== '' && url.value.trim() !== '')

function submit() {
  if (!canSave.value || props.saving) {
    return
  }
  emit('submit', { title: name.value.trim(), url: url.value.trim() })
}
</script>

<template>
  <AppModal title="Add New Status Integration" @close="emit('close')">
    <!-- Enter submits, which is why this is a real form. -->
    <form class="add-integration" @submit.prevent="submit">
      <AppTextField v-model="name" label="Name" />
      <AppTextField v-model="url" label="RSS link" type="url" />
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
