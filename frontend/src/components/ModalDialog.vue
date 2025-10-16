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
          <div>
            <h2 :id="headingId">{{ props.title }}</h2>
            <p v-if="props.description" :id="descriptionId">{{ props.description }}</p>
          </div>
          <button type="button" class="modal__close ghost small" @click="emitClose">
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
  background: rgba(17, 24, 39, 0.55);
  backdrop-filter: blur(4px);
  z-index: 2000;
  overflow-y: auto;
}

.modal {
  width: min(620px, 100%);
  display: flex;
  flex-direction: column;
  background: var(--color-surface);
  border-radius: var(--radius-xl);
  border: 1px solid var(--color-border-soft);
  box-shadow: var(--shadow-elevated);
  padding: 1.75rem;
  gap: 1.5rem;
  animation: modal-in 0.18s ease-out;
}

.modal__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1rem;
}

.modal__header h2 {
  margin: 0;
  font-size: 1.25rem;
  color: var(--color-text-primary);
}

.modal__header p {
  margin: 0.4rem 0 0;
  color: var(--color-text-tertiary);
  font-size: 0.92rem;
}

.modal__close {
  align-self: flex-start;
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
    transform: translateY(-8px);
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
    padding: 1.35rem;
    border-radius: var(--radius-lg);
  }
}
</style>
