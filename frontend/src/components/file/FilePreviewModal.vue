<script setup lang="ts">
import { ref, watch, computed } from 'vue';
import { X, ExternalLink, Copy, Loader2, Check } from 'lucide-vue-next';
import { useToastStore } from '@/stores/toast';

const props = defineProps<{
  isOpen: boolean;
  url: string;
  name: string;
  mimeType: string;
}>();

const emit = defineEmits(['close']);
const toastStore = useToastStore();

const textContent = ref('');
const loading = ref(false);
const error = ref('');
const copied = ref(false);

const isText = computed(() => {
    const mime = props.mimeType.toLowerCase();
    return mime.startsWith('text/') || mime === 'application/json' || mime === 'application/javascript';
});

const isImage = computed(() => {
    return props.mimeType.toLowerCase().startsWith('image/');
});

watch(() => props.isOpen, (isOpen) => {
    if (isOpen) {
        if (isText.value) {
            fetchText();
        } else {
            textContent.value = '';
        }
    }
});

const fetchText = async () => {
    loading.value = true;
    error.value = '';
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 15000);
    try {
        const res = await fetch(props.url, { signal: controller.signal });
        if (!res.ok) throw new Error(`Failed to load content: ${res.statusText}`);
        textContent.value = await res.text();
    } catch (e: any) {
        error.value = e?.name === 'AbortError' ? 'Request timed out' : e.message;
    } finally {
        clearTimeout(timeoutId);
        loading.value = false;
    }
};

const handleCopy = async () => {
    try {
        if (isText.value) {
            await navigator.clipboard.writeText(textContent.value);
            toastStore.success('Text content copied to clipboard');
        } else if (isImage.value) {
             // Fetch blob and copy to clipboard
             const controller = new AbortController();
             const timeoutId = setTimeout(() => controller.abort(), 20000);
             try {
                 const res = await fetch(props.url, { signal: controller.signal });
                 const blob = await res.blob();
                 await navigator.clipboard.write([
                     new ClipboardItem({
                         [blob.type]: blob
                     })
                 ]);
             } finally {
                 clearTimeout(timeoutId);
             }
             toastStore.success('Image copied to clipboard');
        }
        
        copied.value = true;
        setTimeout(() => copied.value = false, 2000);
    } catch (e: any) {
        console.error(e);
        toastStore.error('Failed to copy content');
    }
};
</script>

<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm" @click.self="$emit('close')">
    <div class="bg-white dark:bg-nord-1 rounded-2xl shadow-2xl w-full max-w-5xl h-[85vh] flex flex-col animate-fade-in overflow-hidden">
      
      <!-- Header -->
      <div class="px-6 py-4 border-b border-nord-4 dark:border-nord-2 flex justify-between items-center bg-nord-6 dark:bg-nord-0 shrink-0">
        <h3 class="font-bold text-lg text-nord-1 dark:text-nord-6 truncate max-w-md" :title="name">{{ name }}</h3>
        <div class="flex items-center gap-2">
            
            <!-- Copy Button -->
            <button 
                @click="handleCopy" 
                class="p-2 text-nord-3 dark:text-nord-4 hover:text-nord-10 transition-colors relative" 
                :title="isText ? 'Copy Text' : 'Copy Image'"
            >
                <Check v-if="copied" class="w-5 h-5 text-green-500" />
                <Copy v-else class="w-5 h-5" />
            </button>

            <div class="h-5 w-px bg-nord-4 dark:bg-nord-3 mx-1"></div>

            <a :href="url" target="_blank" class="p-2 text-nord-3 dark:text-nord-4 hover:text-nord-10 transition-colors" title="Open in Browser">
                <ExternalLink class="w-5 h-5" />
            </a>
            <button @click="$emit('close')" class="p-2 text-nord-3 dark:text-nord-4 hover:text-nord-10 transition-colors">
                <X class="w-5 h-5" />
            </button>
        </div>
      </div>

      <!-- Content -->
      <div class="flex-1 bg-nord-6 dark:bg-nord-0 overflow-hidden relative flex items-center justify-center">
          
          <!-- Image -->
          <img v-if="isImage" :src="url" class="max-w-full max-h-full object-contain p-4" />
          
          <!-- Text (Fetched & Rendered) -->
          <div v-else-if="isText" class="w-full h-full overflow-auto bg-white dark:bg-nord-1 p-4">
              <div v-if="loading" class="flex flex-col items-center justify-center h-full text-nord-3 dark:text-nord-4">
                  <Loader2 class="w-8 h-8 animate-spin mb-2" />
                  <span>Loading content...</span>
              </div>
              <div v-else-if="error" class="flex flex-col items-center justify-center h-full text-red-500">
                  <p>Error loading content</p>
                  <p class="text-sm opacity-70">{{ error }}</p>
              </div>
              <pre v-else class="font-mono text-sm text-nord-1 dark:text-nord-6 whitespace-pre-wrap break-words">{{ textContent }}</pre>
          </div>

          <!-- Fallback (Iframe) -->
          <iframe v-else :src="url" class="w-full h-full border-none bg-white"></iframe>
      </div>
      
    </div>
  </div>
</template>

<style scoped>
.animate-fade-in {
  animation: fadeIn 0.2s ease-out;
}
@keyframes fadeIn {
  from { opacity: 0; transform: scale(0.99); }
  to { opacity: 1; transform: scale(1); }
}
</style>
