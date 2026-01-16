<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { Download, Trash2, History } from 'lucide-vue-next';
import { useSharedStore } from '@/stores/shared';
import { useToastStore } from '@/stores/toast';
import { useTaskStore } from '@/stores/tasks';
import { useSettingsStore } from '@/stores/settings';
import { storeToRefs } from 'pinia';
import FileTable from '@/components/file/FileTable.vue';
import SharedFileModal from '@/components/file/SharedFileModal.vue';
import api from '@/api';

const sharedStore = useSharedStore();
const toastStore = useToastStore();
const taskStore = useTaskStore();
const settingsStore = useSettingsStore();

const { history, loading } = storeToRefs(sharedStore);
const sharedInput = ref('');
const showSharedModal = ref(false);
const sharedModalData = ref<any>(null);

onMounted(() => {
    sharedStore.fetchHistory();
});

const handleImportShared = async () => {
    try {
        let cid = '';
        let name = '';

        try {
            const data = JSON.parse(sharedInput.value);
            if (data.cid) {
                cid = data.cid;
                name = data.name || '';
            } else {
                throw new Error("Invalid JSON");
            }
        } catch {
            // Treat as raw CID/String
            if (sharedInput.value.length > 5 && !sharedInput.value.includes(' ')) {
                cid = sharedInput.value;
            } else if (sharedInput.value.startsWith('http')) {
                 window.open(sharedInput.value, '_blank');
                 return;
            } else {
                 throw new Error("Invalid format");
            }
        }

        if (!cid) throw new Error("Invalid share data: missing CID");
        
        // Add to history
        const file = await sharedStore.addToHistory(cid, name);
        toastStore.success('Added to history');
        
        // Open Modal
        sharedModalData.value = file;
        showSharedModal.value = true;
        
        // Clear input
        sharedInput.value = '';
        
    } catch (e: any) {
         toastStore.error(e.message || 'Invalid share format');
    }
};

const handlePreview = (file: any) => {
    let url = `http://localhost:3666/api/preview/${file.cid}`;
    window.open(url, '_blank');
};

const handleDownload = async (file: any) => {
    const filename = file.name || file.cid;
    
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
                    const url = `${baseUrl}/preview/${file.cid}?download=true`;
                    
                    const response = await fetch(url);
                    if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
                    
                    // @ts-ignore
                    await response.body.pipeTo(writable);
                    
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
            console.warn("FileSystemAccess API not supported or failed", err);
        }
    }

    // Backend Silent Download
    if (!settingsStore.askPath && settingsStore.downloadPath) {
        const taskId = taskStore.addTask('download', filename);
        toastStore.success(`Download started for ${filename}`);
        
        try {
            await api.post('/files/download/shared', {
                cid: file.cid,
                name: filename,
            });
            taskStore.completeTask(taskId);
        } catch (e: any) {
             taskStore.failTask(taskId, e.message || 'Download failed');
             toastStore.error(`Download failed: ${e.message}`);
        }
        return;
    }

    // Fallback: Browser Blob Download
    const taskId = taskStore.addTask('download', filename);
    toastStore.success('Download started');
    
    try {
        const url = `/preview/${file.cid}?download=true`;
        const response = await api.get(url, {
            responseType: 'blob',
            onDownloadProgress: (progressEvent) => {
                if (progressEvent.total) {
                    taskStore.updateProgress(taskId, progressEvent.loaded, progressEvent.total);
                }
            }
        });
        
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
        taskStore.failTask(taskId, e.message || 'Download failed');
        toastStore.error(`Download failed: ${e.message}`);
    }
};

const handleShare = (file: any) => {
    // Copy CID to clipboard
    navigator.clipboard.writeText(file.cid);
    toastStore.success('CID copied to clipboard');
};

const handlePin = async (file: any) => {
    const filename = file.name || file.cid;
    const taskId = taskStore.addTask('download', `Pinning ${filename}...`); // Use download type for icon/progress semantics if needed, or generic
    toastStore.success(`Pinning started for ${filename}`);
    
    try {
        await api.post('/shared/pin', { cid: file.cid });
        taskStore.completeTask(taskId);
        toastStore.success(`${filename} pinned and added to My Files`);
    } catch (e: any) {
        console.error(e);
        taskStore.failTask(taskId, e.message || 'Pin failed');
        toastStore.error(`Pin failed: ${e.message}`);
    }
};

const handleDelete = async (id: number) => {
    if (confirm('Remove this item from history?')) {
        await sharedStore.deleteHistory(id);
        toastStore.success('Item removed');
    }
};

const handleClearHistory = async () => {
    if (confirm('Clear all shared file history?')) {
        await sharedStore.clearHistory();
        toastStore.success('History cleared');
    }
};

const onModalPreview = () => {
    if (sharedModalData.value) {
        handlePreview(sharedModalData.value);
    }
};

const onModalDownload = () => {
    if (sharedModalData.value) {
        handleDownload(sharedModalData.value);
        showSharedModal.value = false;
    }
};

const onModalPin = () => {
    if (sharedModalData.value) {
        handlePin(sharedModalData.value);
        showSharedModal.value = false;
    }
};
</script>

<template>
    <div class="h-full flex flex-col p-6 gap-6 animate-fade-in">
        
        <!-- Top Section: Compact Import -->
        <div class="w-full bg-white dark:bg-nord-1 p-4 rounded-xl shadow-sm flex items-center gap-4">
             <input 
                v-model="sharedInput"
                type="text"
                placeholder="Paste CID or JSON here to import..."
                class="flex-1 px-4 py-2.5 rounded-lg border border-nord-4 dark:border-nord-3 bg-nord-6 dark:bg-nord-0 text-nord-1 dark:text-nord-6 focus:ring-2 focus:ring-nord-10 outline-none transition-all font-mono text-sm"
                @keyup.enter="handleImportShared"
             />
             <button 
                @click="handleImportShared"
                :disabled="!sharedInput"
                class="px-6 py-2.5 bg-nord-10 hover:bg-nord-9 disabled:opacity-50 disabled:cursor-not-allowed text-white font-bold rounded-lg transition-colors shadow-lg shadow-nord-10/20 whitespace-nowrap"
             >
                View File
             </button>
        </div>

        <!-- Bottom Section: History List (Clean) -->
        <div class="flex-1 flex flex-col min-h-0 overflow-hidden">
            <div class="flex-1 overflow-auto">
                <div v-if="loading" class="flex justify-center py-12">
                    <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-nord-10"></div>
                </div>
                <FileTable 
                    v-else
                    :files="history" 
                    :show-clear-history="true"
                    :show-pin="true"
                    @preview="handlePreview"
                    @share="handleShare"
                    @download="handleDownload"
                    @delete="handleDelete"
                    @clear-history="handleClearHistory"
                    @pin="handlePin"
                />
            </div>
        </div>

        <SharedFileModal 
            :is-open="showSharedModal"
            :shared-data="sharedModalData"
            @close="showSharedModal = false"
            @preview="onModalPreview"
            @download="onModalDownload"
            @pin="onModalPin"
        />

    </div>
</template>
