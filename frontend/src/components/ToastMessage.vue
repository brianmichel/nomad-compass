<template>
  <transition name="toast-message">
    <div class="toast toast-end toast-bottom notification-toast" role="status">
      <div class="alert alert-error">
        <span>{{ message }}</span>
        <button class="btn btn-sm btn-circle btn-ghost" type="button" aria-label="Dismiss notification" @click="$emit('dismiss')">
          <IconX aria-hidden="true" />
        </button>
      </div>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { IconX } from '@tabler/icons-vue';
defineProps<{ message: string }>();
defineEmits<{ (e: 'dismiss'): void }>();
</script>

<style scoped>
.notification-toast { z-index: 2100; }
.alert { grid-template-columns: minmax(0, 1fr) auto; max-width: min(28rem, calc(100vw - 2rem)); font-size: .8rem; }
.alert svg { width: .9rem; }
.toast-message-enter-active, .toast-message-leave-active { transition: opacity var(--transition-base), transform var(--transition-base); }
.toast-message-enter-from, .toast-message-leave-to { opacity: 0; transform: translateY(.5rem); }
</style>
