<script setup lang="ts">
import { useToastStore } from '@/stores/toast';
import { X, CheckCircle, AlertCircle, Info } from 'lucide-vue-next';

const toastStore = useToastStore();
</script>

<template>
  <div class="fixed bottom-4 right-4 z-[100] flex flex-col gap-2">
    <transition-group name="toast">
      <div 
        v-for="toast in toastStore.toasts" 
        :key="toast.id"
        class="flex items-center gap-3 px-4 py-3 rounded-xl shadow-lg border backdrop-blur-md min-w-[300px] transition-all"
        :class="{
          'bg-white/90 border-nord-4 text-nord-1 dark:bg-nord-1/90 dark:border-nord-2 dark:text-nord-6': true
        }"
      >
        <CheckCircle v-if="toast.type === 'success'" class="w-5 h-5 text-green-500" />
        <AlertCircle v-else-if="toast.type === 'error'" class="w-5 h-5 text-red-500" />
        <Info v-else class="w-5 h-5 text-nord-10" />
        
        <p class="text-sm font-medium flex-1">{{ toast.message }}</p>
        
        <button @click="toastStore.remove(toast.id)" class="text-nord-3 hover:text-nord-1 dark:text-nord-4 dark:hover:text-white">
          <X class="w-4 h-4" />
        </button>
      </div>
    </transition-group>
  </div>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}
.toast-enter-from {
  opacity: 0;
  transform: translateX(30px);
}
.toast-leave-to {
  opacity: 0;
  transform: translateY(30px);
}
</style>
