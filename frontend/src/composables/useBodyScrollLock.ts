import { onBeforeUnmount, watch } from 'vue';

export function useBodyScrollLock(source: { value: boolean }) {
  if (typeof document === 'undefined') {
    return;
  }

  const stop = watch(
    () => source.value,
    (value) => {
      document.body.style.overflow = value ? 'hidden' : '';
    },
    { immediate: true },
  );

  onBeforeUnmount(() => {
    stop();
    document.body.style.overflow = '';
  });
}
