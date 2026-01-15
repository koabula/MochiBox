<script setup lang="ts">
import { Copy, X, Shield } from 'lucide-vue-next';
import { useToastStore } from '@/stores/toast';

const props = defineProps<{
  isOpen: boolean;
  file: any;
}>();

const emit = defineEmits(['close']);
const toast = useToastStore();

const handleCopy = async () => {
    if (!props.file) return;

    let shareData: any = {
        cid: props.file.cid,
        name: props.file.name,
        size: props.file.size,
    };
    
    try {
        await navigator.clipboard.writeText(JSON.stringify(shareData, null, 2));
        toast.success('Share info copied to clipboard!');
        emit('close');
    } catch (e) {
        toast.error('Failed to copy');
    }
};

const close = () => {
    emit('close');
};
</script>

<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
    <div class="bg-white dark:bg-nord-1 rounded-2xl shadow-xl w-full max-w-md overflow-hidden">
      
      <div class="px-6 py-4 border-b border-nord-4 dark:border-nord-2 flex justify-between items-center bg-nord-6 dark:bg-nord-2">
        <h3 class="font-bold text-lg text-nord-1 dark:text-nord-6 flex items-center gap-2">
            <Shield class="w-5 h-5 text-nord-10" /> Share File
        </h3>
        <button @click="close" class="text-nord-3 hover:text-nord-1 dark:text-nord-4 dark:hover:text-white">
          <X class="w-5 h-5" />
        </button>
      </div>

      <div class="p-6 space-y-6">
          <div class="p-4 bg-nord-6 dark:bg-nord-2 rounded-xl">
              <h4 class="font-bold text-nord-1 dark:text-nord-6 truncate">{{ file.name }}</h4>
              <p class="text-xs font-mono text-nord-3 dark:text-nord-4 mt-1">{{ file.cid }}</p>
          </div>

          <button 
            @click="handleCopy"
            class="w-full py-3 bg-nord-10 hover:bg-nord-9 disabled:opacity-50 disabled:cursor-not-allowed text-white font-bold rounded-xl transition-colors shadow-lg shadow-nord-10/20 flex items-center justify-center gap-2"
          >
            <Copy class="w-4 h-4" /> Copy Share JSON
          </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.animate-fade-in {
  animation: fadeIn 0.2s ease-out;
}
@keyframes fadeIn {
  from { opacity: 0; transform: translateY(-5px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>