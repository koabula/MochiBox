<script setup lang="ts">
import { ref } from 'vue';
import { useToastStore } from '@/stores/toast';
import { useTaskStore } from '@/stores/tasks';
import { useSettingsStore } from '@/stores/settings';
import api from '@/api';
import { Download, Eye, FileText, X } from 'lucide-vue-next';

const props = defineProps<{
  isOpen: boolean;
  sharedData: any;
}>();

const emit = defineEmits(['close']);
const toastStore = useToastStore();
const taskStore = useTaskStore();
const settingsStore = useSettingsStore();
const loading = ref(false);
const error = ref('');
const previewContent = ref<string | null>(null);
const previewType = ref<string>('');
// ... (rest of code)
const handlePreview = async () => {
    loading.value = true;
    error.value = '';
    previewContent.value = null;
    
    try {
        const response = await api.get(`/preview/${props.sharedData.cid}`, { responseType: 'blob' });
        const type = response.headers['content-type'];
        previewType.value = type;
        
        if (type.startsWith('image/') || type.startsWith('video/') || type === 'application/pdf') {
            previewContent.value = URL.createObjectURL(response.data);
        } else if (type.startsWith('text/') || type === 'application/json') {
            previewContent.value = await response.data.text();
        } else {
            error.value = 'Preview not supported for this file type';
        }
    } catch (e: any) {
        error.value = 'Failed to load preview: ' + (e.response?.data?.error || e.message);
    } finally {
        loading.value = false;
    }
};

const handleDownload = async () => {
    const filename = props.sharedData.name || props.sharedData.cid;
    
    // Auto-close modal
    closeModal();

    // Check settings for "Always ask" or missing path
    if (settingsStore.askPath || !settingsStore.downloadPath) {
        try {
            // @ts-ignore
            if (window.showSaveFilePicker) {
                // @ts-ignore
                const handle = await window.showSaveFilePicker({
                    suggestedName: filename
                });
                
                const taskId = taskStore.addTask('download', filename);
                toastStore.success(`Download started for ${filename}`);
                
                try {
                    const writable = await handle.createWritable();
                    const baseUrl = api.defaults.baseURL || 'http://localhost:3666/api';
                    const url = `${baseUrl}/preview/${props.sharedData.cid}?download=true`;
                    
                    const response = await fetch(url);
                    if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
                    if (!response.body) throw new Error("No response body");
                    
                    const total = Number(response.headers.get('content-length')) || 0;
                    let loaded = 0;
                    
                    const reader = response.body.getReader();
                    
                    while (true) {
                        const { done, value } = await reader.read();
                        if (done) break;
                        loaded += value.length;
                        if (total) taskStore.updateProgress(taskId, loaded, total);
                        await writable.write(value);
                    }
                    
                    await writable.close();
                    taskStore.completeTask(taskId);
                    return; 
                    
                } catch (e: any) {
                    console.error(e);
                    taskStore.failTask(taskId, e.message);
                    toastStore.error(`Download failed: ${e.message}`);
                    return;
                }
            }
        } catch (err: any) {
            if (err.name === 'AbortError') return;
            console.warn("FileSystemAccess API not supported or failed, falling back to default download", err);
        }
    }

    // If settingsStore.downloadPath is set and "Ask" is false, we use the Backend to save silently
    if (!settingsStore.askPath && settingsStore.downloadPath) {
        const taskId = taskStore.addTask('download', filename);
        toastStore.success(`Download started for ${filename}`);
        
        try {
            // Use backend to download
            await api.post('/files/download/shared', {
                cid: props.sharedData.cid,
                name: filename,
            });
            taskStore.completeTask(taskId);
        } catch (e: any) {
             console.error(e);
             taskStore.failTask(taskId, e.message || 'Download failed');
             toastStore.error(`Download failed: ${e.message}`);
        }
        return;
    }

    const taskId = taskStore.addTask('download', filename);
    toastStore.success('Download started');
    
    try {
        const url = `/preview/${props.sharedData.cid}?download=true`;
        
        const response = await api.get(url, {
            responseType: 'blob',
            onDownloadProgress: (progressEvent) => {
                if (progressEvent.total) {
                    taskStore.updateProgress(taskId, progressEvent.loaded, progressEvent.total);
                }
            }
        });
        
        // Trigger save
        const blob = new Blob([response.data], { type: response.headers['content-type'] });
        const downloadUrl = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = downloadUrl;
        link.download = filename; 
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        window.URL.revokeObjectURL(downloadUrl);
        
        taskStore.completeTask(taskId);
    } catch (e: any) {
        console.error(e);
        taskStore.failTask(taskId, e.message || 'Download failed');
        toastStore.error(`Download failed: ${e.message}`);
    }
};

const closeModal = () => {
    error.value = '';
    previewContent.value = null;
    emit('close');
};
</script>

<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm">
    <div class="bg-white dark:bg-nord-1 rounded-2xl shadow-2xl w-full max-w-lg overflow-hidden flex flex-col max-h-[90vh]">
      
      <!-- Header -->
      <div class="px-6 py-4 border-b border-nord-4 dark:border-nord-2 flex justify-between items-center bg-nord-6 dark:bg-nord-0">
        <h3 class="font-bold text-lg text-nord-1 dark:text-nord-6 flex items-center gap-2">
            <Download class="w-5 h-5 text-nord-10" />
            Import Shared File
        </h3>
        <button @click="closeModal" class="text-nord-3 dark:text-nord-4 hover:text-nord-10 transition-colors">
            <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Body -->
      <div class="p-6 overflow-y-auto space-y-6">
        
        <!-- File Info -->
        <div class="flex items-start gap-4 p-4 bg-nord-6 dark:bg-nord-2 rounded-xl">
            <div class="p-3 bg-white dark:bg-nord-3 rounded-lg shadow-sm">
                <FileText class="w-8 h-8 text-nord-10" />
            </div>
            <div class="flex-1 min-w-0">
                <h4 class="font-bold text-nord-1 dark:text-nord-6 truncate">{{ sharedData.name || 'Unknown Filename' }}</h4>
                <p class="text-xs font-mono text-nord-3 dark:text-nord-4 truncate" :title="sharedData.cid">CID: {{ sharedData.cid }}</p>
            </div>
        </div>

        <!-- Preview Area -->
        <div v-if="previewContent" class="mt-4 p-4 border border-nord-4 dark:border-nord-2 rounded-xl bg-nord-6 dark:bg-nord-0 overflow-auto" :class="previewType === 'application/pdf' ? 'h-[500px]' : 'max-h-60'">
             <img v-if="previewType.startsWith('image/')" :src="previewContent" class="max-w-full h-auto rounded-lg mx-auto" />
             <video v-else-if="previewType.startsWith('video/')" :src="previewContent" controls class="max-w-full h-auto rounded-lg mx-auto"></video>
             <iframe v-else-if="previewType === 'application/pdf'" :src="previewContent" class="w-full h-full rounded-lg border-0"></iframe>
             <pre v-else class="text-xs font-mono whitespace-pre-wrap text-nord-1 dark:text-nord-5">{{ previewContent }}</pre>
        </div>
        
        <!-- Error Message -->
        <div v-if="error" class="text-sm text-red-500 bg-red-50 dark:bg-red-900/10 p-3 rounded-lg">
            {{ error }}
        </div>

      </div>

      <!-- Footer -->
      <div class="p-6 border-t border-nord-4 dark:border-nord-2 bg-nord-6 dark:bg-nord-0 flex gap-3">
        <button 
            @click="handlePreview"
            :disabled="loading"
            class="flex-1 py-2.5 px-4 bg-white dark:bg-nord-3 border border-nord-4 dark:border-nord-2 hover:bg-nord-5 dark:hover:bg-nord-2 text-nord-1 dark:text-nord-6 font-medium rounded-xl transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
        >
            <Eye class="w-4 h-4" /> {{ loading ? 'Loading...' : 'Preview' }}
        </button>
        <button 
            @click="handleDownload"
            class="flex-1 py-2.5 px-4 bg-nord-10 hover:bg-nord-9 text-white font-medium rounded-xl transition-colors shadow-lg shadow-nord-10/20 disabled:opacity-50 flex items-center justify-center gap-2"
        >
            <Download class="w-4 h-4" /> Download
        </button>
      </div>

    </div>
  </div>
</template>