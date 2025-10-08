<template>
  <transition name="toast">
    <div class="toast">
      <span>{{ message }}</span>
      <button class="ghost" type="button" @click="$emit('dismiss')">Dismiss</button>
    </div>
  </transition>
</template>

<script setup lang="ts">
defineProps<{ message: string }>();
</script>
