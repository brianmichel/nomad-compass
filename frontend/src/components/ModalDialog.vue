<template>
  <teleport to="body" v-if="props.open">
    <div class="modal-backdrop" @click.self="emitClose">
      <div
        class="modal"
        role="dialog"
        :aria-labelledby="headingId"
        :aria-describedby="props.description ? descriptionId : undefined"
        aria-modal="true"
      >
        <header class="modal__header">
          <div class="modal__heading">
            <h2 :id="headingId">{{ props.title }}</h2>
            <p v-if="props.description" :id="descriptionId">{{ props.description }}</p>
          </div>
          <button
            type="button"
            class="modal__close"
            aria-label="Close dialog"
            @click="emitClose"
          >
            <span aria-hidden="true">&times;</span>
            <span class="sr-only">Close</span>
          </button>
        </header>
        <div class="modal__body">
          <slot />
        </div>
        <footer v-if="$slots.footer" class="modal__footer">
          <slot name="footer" />
        </footer>
      </div>
    </div>
  </teleport>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue';

let modalIdCounter = 0;

const props = defineProps<{
  open: boolean;
  title: string;
  description?: string;
}>();

const emit = defineEmits<{
  (e: 'close'): void;
}>();

const headingId = `modal-${++modalIdCounter}`;
const descriptionId = `${headingId}-description`;
const previousOverflow = ref('');

const emitClose = () => {
  emit('close');
};

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    emitClose();
  }
}

watch(
  () => props.open,
  (value) => {
    if (value) {
      previousOverflow.value = document.body.style.overflow;
      document.body.style.overflow = 'hidden';
      document.addEventListener('keydown', handleKeydown);
    } else {
      document.body.style.overflow = previousOverflow.value;
      document.removeEventListener('keydown', handleKeydown);
    }
  },
  { immediate: true }
);

onBeforeUnmount(() => {
  document.body.style.overflow = previousOverflow.value;
  document.removeEventListener('keydown', handleKeydown);
});
</script>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 4rem 1.25rem;
  background: rgba(24, 37, 58, 0.2);
  backdrop-filter: blur(2px);
  z-index: 2000;
  overflow-y: auto;
}

.modal {
  width: min(680px, 100%);
  display: flex;
  flex-direction: column;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98) 0%, rgba(248, 251, 255, 0.96) 100%);
  border-radius: 14px;
  border: 1px solid var(--color-border-soft);
  box-shadow:
    0 24px 48px -34px rgba(15, 23, 42, 0.28),
    0 10px 22px -18px rgba(15, 23, 42, 0.14);
  padding: 1.4rem 1.4rem 1.5rem;
  gap: 1.25rem;
  animation: modal-in 0.16s ease-out;
}

.modal__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
  padding-bottom: 0.15rem;
}

.modal__heading {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.modal__header h2 {
  margin: 0;
  font-size: 1.2rem;
  line-height: 1.2;
  color: var(--color-text-primary);
}

.modal__header p {
  margin: 0;
  max-width: 34rem;
  color: var(--color-text-secondary);
  font-size: 0.92rem;
}

.modal__close {
  flex: 0 0 auto;
  justify-content: center;
  width: 2.2rem;
  height: 2.2rem;
  padding: 0;
  border-radius: 999px;
  border: 1px solid rgba(193, 199, 211, 0.95);
  background: rgba(247, 249, 252, 0.92);
  color: var(--color-text-tertiary);
  line-height: 1;
  box-shadow: 0 6px 14px -12px rgba(15, 23, 42, 0.2);
}

.modal__close:hover,
.modal__close:focus-visible {
  background: var(--color-surface);
  border-color: var(--color-border-strong);
  color: var(--color-text-primary);
  transform: translateY(-1px);
}

.modal__body {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.modal__footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

@keyframes modal-in {
  from {
    opacity: 0;
    transform: translateY(-6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 640px) {
  .modal-backdrop {
    padding: 3rem 0.75rem;
  }

  .modal {
    padding: 1.15rem 1.15rem 1.25rem;
    border-radius: 12px;
  }
}
</style>
