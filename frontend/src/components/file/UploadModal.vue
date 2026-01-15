<script setup lang="ts">
import { ref } from 'vue';
import { X, Upload, File as FileIcon } from 'lucide-vue-next';

const props = defineProps<{
  isOpen: boolean
}>();

const emit = defineEmits(['close', 'upload']);

const fileInput = ref<HTMLInputElement | null>(null);
const selectedFile = ref<File | null>(null);

const handleFileSelect = (event: Event) => {
  const target = event.target as HTMLInputElement;
  if (target.files && target.files.length > 0) {
    selectedFile.value = target.files[0];
  }
};

const handleDrop = (event: DragEvent) => {
  event.preventDefault();
  if (event.dataTransfer?.files && event.dataTransfer.files.length > 0) {
    selectedFile.value = event.dataTransfer.files[0];
  }
};

const handleSubmit = () => {
  if (!selectedFile.value) return;
  emit('upload', { 
      file: selectedFile.value
  });
  reset();
};

const reset = () => {
  selectedFile.value = null;
};

const close = () => {
    reset();
    emit('close');
}
</script>

<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm transition-opacity">
    <div class="bg-white dark:bg-nord-1 rounded-2xl shadow-xl w-full max-w-md overflow-hidden transform transition-all scale-100">
      
      <div class="px-6 py-4 border-b border-nord-4 dark:border-nord-2 flex justify-between items-center bg-nord-6 dark:bg-nord-2">
        <h3 class="font-bold text-lg text-nord-1 dark:text-nord-6">Upload File</h3>
        <button @click="close" class="text-nord-3 hover:text-nord-1 dark:text-nord-4 dark:hover:text-white">
          <X class="w-5 h-5" />
        </button>
      </div>

      <div class="p-6 space-y-6">
        
        <!-- Drop Zone -->
        <div 
          v-if="!selectedFile"
          @dragover.prevent
          @drop="handleDrop"
          @click="fileInput?.click()"
          class="border-2 border-dashed border-nord-4 dark:border-nord-3 rounded-xl p-8 flex flex-col items-center justify-center gap-3 cursor-pointer hover:border-nord-10 dark:hover:border-nord-8 hover:bg-nord-6 dark:hover:bg-nord-2 transition-all"
        >
          <div class="p-3 bg-nord-5 dark:bg-nord-3 rounded-full">
            <Upload class="w-6 h-6 text-nord-10 dark:text-nord-8" />
          </div>
          <div class="text-center">
            <p class="font-medium text-nord-1 dark:text-nord-6">Click or drag file to upload</p>
            <p class="text-xs text-nord-3 dark:text-nord-4 mt-1">Any file type supported</p>
          </div>
          <input ref="fileInput" type="file" class="hidden" @change="handleFileSelect" />
        </div>

        <!-- Selected File Preview -->
        <div v-else class="flex items-center gap-4 p-4 bg-nord-6 dark:bg-nord-2 rounded-lg border border-nord-4 dark:border-nord-3">
          <div class="p-2 bg-nord-4 dark:bg-nord-3 rounded">
            <FileIcon class="w-6 h-6 text-nord-10" />
          </div>
          <div class="flex-1 min-w-0">
            <p class="font-medium text-sm truncate text-nord-1 dark:text-nord-6">{{ selectedFile.name }}</p>
            <p class="text-xs text-nord-3 dark:text-nord-4">{{ (selectedFile.size / 1024).toFixed(1) }} KB</p>
          </div>
          <button @click="selectedFile = null" class="text-nord-3 hover:text-red-500">
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Submit Button -->
        <button 
          @click="handleSubmit"
          :disabled="!selectedFile"
          class="w-full py-3 bg-nord-10 hover:bg-nord-9 disabled:opacity-50 disabled:cursor-not-allowed text-white font-bold rounded-xl transition-colors shadow-lg shadow-nord-10/20"
        >
          Upload to IPFS
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