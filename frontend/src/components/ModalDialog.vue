<template>
  <teleport v-if="props.open" to="body">
    <div class="modal modal-open">
      <div
        ref="dialogRef"
        class="modal-box dialog-box"
        role="dialog"
        tabindex="-1"
        :aria-labelledby="headingId"
        :aria-describedby="props.description ? descriptionId : undefined"
        aria-modal="true"
      >
        <header class="dialog-header">
          <div>
            <h2 :id="headingId">{{ props.title }}</h2>
            <p v-if="props.description" :id="descriptionId">{{ props.description }}</p>
          </div>
          <button type="button" class="btn btn-sm btn-circle btn-ghost" aria-label="Close dialog" @click="emitClose">
            <IconX aria-hidden="true" />
          </button>
        </header>
        <div class="dialog-body"><slot /></div>
        <footer v-if="$slots.footer" class="modal-action"><slot name="footer" /></footer>
      </div>
      <button class="modal-backdrop" type="button" aria-label="Close dialog" @click="emitClose">Close</button>
    </div>
  </teleport>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { IconX } from '@tabler/icons-vue';

let modalIdCounter = 0;
const props = defineProps<{ open: boolean; title: string; description?: string }>();
const emit = defineEmits<{ (e: 'close'): void }>();
const headingId = `modal-${++modalIdCounter}`;
const descriptionId = `${headingId}-description`;
const previousOverflow = ref('');
const dialogRef = ref<HTMLElement | null>(null);
let previouslyFocusedElement: HTMLElement | null = null;

function emitClose(): void {
  emit('close');
}

function handleKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') {
    emitClose();
    return;
  }
  if (event.key !== 'Tab' || !dialogRef.value) return;

  const focusable = Array.from(dialogRef.value.querySelectorAll<HTMLElement>(
    'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
  ));
  if (!focusable.length) {
    event.preventDefault();
    dialogRef.value.focus();
    return;
  }

  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  if (!dialogRef.value.contains(document.activeElement)) {
    event.preventDefault();
    (event.shiftKey ? last : first).focus();
  } else if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}

watch(
  () => props.open,
  async (value) => {
    if (value) {
      previouslyFocusedElement = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      previousOverflow.value = document.body.style.overflow;
      document.body.style.overflow = 'hidden';
      document.addEventListener('keydown', handleKeydown);
      await nextTick();
      const explicitFocus = dialogRef.value?.querySelector<HTMLElement>('[data-autofocus]');
      const formFocus = dialogRef.value?.querySelector<HTMLElement>('input:not([type="hidden"]), select, textarea');
      const fallbackFocus = dialogRef.value?.querySelector<HTMLElement>('button, a[href]');
      (explicitFocus ?? formFocus ?? fallbackFocus ?? dialogRef.value)?.focus();
    } else {
      document.body.style.overflow = previousOverflow.value;
      document.removeEventListener('keydown', handleKeydown);
      await nextTick();
      if (previouslyFocusedElement?.isConnected) {
        previouslyFocusedElement.focus();
      } else {
        document.querySelector<HTMLElement>('[data-dialog-fallback]')?.focus();
      }
      previouslyFocusedElement = null;
    }
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  document.body.style.overflow = previousOverflow.value;
  document.removeEventListener('keydown', handleKeydown);
  previouslyFocusedElement?.focus();
});
</script>

<style scoped>
.dialog-box { width: min(42rem, calc(100vw - 2rem)); max-width: 42rem; padding: 1.25rem; border: 1px solid var(--color-border); border-radius: var(--radius-box); background: var(--color-surface); box-shadow: var(--shadow-elevated); }
.dialog-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; }
.dialog-header h2 { margin: 0; color: var(--color-text-primary); font-size: 1.08rem; font-weight: 680; }
.dialog-header p { max-width: 34rem; margin: .25rem 0 0; color: var(--color-text-tertiary); font-size: .82rem; }
.dialog-header .btn { flex: 0 0 auto; margin: -.25rem -.25rem 0 0; }
.dialog-header svg { width: 1rem; }
.dialog-body { display: flex; flex-direction: column; gap: 1rem; margin-top: 1.15rem; }
.modal-action { margin-top: 1.25rem; }
@media (max-width: 640px) { .dialog-box { width: calc(100vw - 1rem); padding: 1rem; } }
</style>
