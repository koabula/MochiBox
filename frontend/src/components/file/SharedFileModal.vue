<script setup lang="ts">
import { Download, Eye, FileText, X, Pin } from 'lucide-vue-next';

defineProps<{
  isOpen: boolean;
  sharedData: any;
}>();

const emit = defineEmits(['close', 'preview', 'download', 'pin']);
</script>

<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm">
    <div class="bg-white dark:bg-nord-1 rounded-2xl shadow-2xl w-full max-w-md overflow-hidden flex flex-col animate-fade-in">
      
      <!-- Header -->
      <div class="px-6 py-4 border-b border-nord-4 dark:border-nord-2 flex justify-between items-center bg-nord-6 dark:bg-nord-0">
        <h3 class="font-bold text-lg text-nord-1 dark:text-nord-6 flex items-center gap-2">
            <Download class="w-5 h-5 text-nord-10" />
            Imported File
        </h3>
        <button @click="$emit('close')" class="text-nord-3 dark:text-nord-4 hover:text-nord-10 transition-colors">
            <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Body -->
      <div class="p-6 space-y-6">
        
        <!-- File Info -->
        <div class="flex items-start gap-4 p-4 bg-nord-6 dark:bg-nord-2 rounded-xl">
            <div class="p-3 bg-white dark:bg-nord-3 rounded-lg shadow-sm">
                <FileText class="w-8 h-8 text-nord-10" />
            </div>
            <div class="flex-1 min-w-0">
                <h4 class="font-bold text-nord-1 dark:text-nord-6 truncate">{{ sharedData?.name || 'Unknown Filename' }}</h4>
                <p class="text-xs font-mono text-nord-3 dark:text-nord-4 truncate" :title="sharedData?.cid">CID: {{ sharedData?.cid }}</p>
                <p v-if="sharedData?.size" class="text-xs text-nord-3 dark:text-nord-4 mt-1">{{ sharedData.size }}</p>
            </div>
        </div>

        <!-- Actions -->
        <div class="flex gap-3">
            <button 
                @click="$emit('preview')"
                class="flex-1 py-3 px-4 bg-white dark:bg-nord-3 border border-nord-4 dark:border-nord-2 hover:bg-nord-5 dark:hover:bg-nord-2 text-nord-1 dark:text-nord-6 font-medium rounded-xl transition-colors flex items-center justify-center gap-2"
            >
                <Eye class="w-4 h-4" /> Preview
            </button>
            <button 
                @click="$emit('download')"
                class="flex-1 py-3 px-4 bg-nord-10 hover:bg-nord-9 text-white font-medium rounded-xl transition-colors shadow-lg shadow-nord-10/20 flex items-center justify-center gap-2"
            >
                <Download class="w-4 h-4" /> Download
            </button>
            <button 
                @click="$emit('pin')"
                class="py-3 px-4 bg-white dark:bg-nord-3 border border-nord-4 dark:border-nord-2 hover:bg-nord-5 dark:hover:bg-nord-2 text-nord-1 dark:text-nord-6 font-medium rounded-xl transition-colors flex items-center justify-center gap-2"
                title="Pin to Local Node"
            >
                <Pin class="w-4 h-4" />
            </button>
        </div>

      </div>

    </div>
  </div>
</template>

<style scoped>
.animate-fade-in {
  animation: fadeIn 0.2s ease-out;
}
@keyframes fadeIn {
  from { opacity: 0; transform: scale(0.95); }
  to { opacity: 1; transform: scale(1); }
}
</style>
