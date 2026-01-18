<script setup lang="ts">
import { ref, watch } from 'vue';
import { X, Lock } from 'lucide-vue-next';

const props = defineProps<{
  isOpen: boolean;
  fileName: string;
}>();

const emit = defineEmits(['close', 'submit']);

const password = ref('');

watch(() => props.isOpen, (newVal) => {
    if (newVal) password.value = '';
});

const handleSubmit = () => {
    if (!password.value) return;
    emit('submit', password.value);
};
</script>

<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm transition-opacity">
    <div class="bg-white dark:bg-nord-1 rounded-2xl shadow-xl w-full max-w-sm overflow-hidden transform transition-all scale-100">
      
      <div class="px-6 py-4 border-b border-nord-4 dark:border-nord-2 flex justify-between items-center bg-nord-6 dark:bg-nord-2">
        <h3 class="font-bold text-lg text-nord-1 dark:text-nord-6 flex items-center gap-2">
            <Lock class="w-5 h-5 text-nord-10" />
            Enter Password
        </h3>
        <button @click="emit('close')" class="text-nord-3 hover:text-nord-1 dark:text-nord-4 dark:hover:text-white">
          <X class="w-5 h-5" />
        </button>
      </div>

      <div class="p-6 space-y-4">
        <p class="text-sm text-nord-3 dark:text-nord-4">
            Enter password to decrypt <span class="font-bold text-nord-1 dark:text-nord-6">{{ fileName }}</span>
        </p>
        
        <input 
            v-model="password" 
            type="password" 
            placeholder="Password" 
            class="w-full px-3 py-2 rounded-lg border border-nord-4 dark:border-nord-3 bg-white dark:bg-nord-0 text-sm focus:ring-2 focus:ring-nord-10 outline-none text-nord-0 dark:text-nord-6"
            @keyup.enter="handleSubmit"
            autofocus
        />

        <button 
          @click="handleSubmit"
          :disabled="!password"
          class="w-full py-2 bg-nord-10 hover:bg-nord-9 disabled:opacity-50 disabled:cursor-not-allowed text-white font-bold rounded-lg transition-colors shadow-lg shadow-nord-10/20"
        >
          Decrypt & Download
        </button>
      </div>
    </div>
  </div>
</template>