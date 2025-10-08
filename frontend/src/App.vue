<template>
  <div class="app-shell">
    <Topbar :refreshing="refreshing" :status="status" @refresh="refreshAll" />
    <div class="content-frame">
      <router-view />
    </div>
    <ToastMessage v-if="error" :message="error" @dismiss="clearError" />
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';
import Topbar from './components/Topbar.vue';
import ToastMessage from './components/ToastMessage.vue';
import { useCompassStore } from './composables/useCompassStore';

const { refreshAll, refreshing, error, clearError, status } = useCompassStore();

onMounted(() => {
  refreshAll();
});
</script>

<style scoped>
.app-shell {
  min-height: 100vh;
  padding: 2.5rem clamp(1rem, 4vw, 4rem);
  display: flex;
  flex-direction: column;
  gap: 2rem;
  max-width: 1200px;
  margin: 0 auto;
}

.content-frame {
  flex: 1;
}
</style>
