<script setup lang="ts">
import { Minus, Square, X } from 'lucide-vue-next';

const minimize = () => {
  // @ts-ignore
  window.electronAPI?.minimize();
};

const maximize = () => {
  // @ts-ignore
  window.electronAPI?.maximize();
};

const close = () => {
  // @ts-ignore
  window.electronAPI?.close();
};
</script>

<template>
  <div class="flex items-center no-drag h-full">
    <button @click="minimize" class="p-2 hover:bg-nord-4 dark:hover:bg-nord-2 text-nord-3 dark:text-nord-4 transition-colors rounded-lg">
      <Minus class="w-4 h-4" />
    </button>
    <button @click="maximize" class="p-2 hover:bg-nord-4 dark:hover:bg-nord-2 text-nord-3 dark:text-nord-4 transition-colors rounded-lg">
      <Square class="w-4 h-4" />
    </button>
    <button @click="close" class="p-2 hover:bg-red-500 hover:text-white text-nord-3 dark:text-nord-4 transition-colors rounded-lg">
      <X class="w-4 h-4" />
    </button>
  </div>
</template>
