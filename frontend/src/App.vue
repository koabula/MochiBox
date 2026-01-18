<script setup lang="ts">
import { ref, onMounted, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useFileStore } from '@/stores/files';
import { useSettingsStore } from '@/stores/settings';
import { useToastStore } from '@/stores/toast';
import { useNetworkStore } from '@/stores/network';
import { storeToRefs } from 'pinia';
import Sidebar from '@/components/layout/Sidebar.vue';
import FileTable from '@/components/file/FileTable.vue';
import UploadModal from '@/components/file/UploadModal.vue';
import SharedPage from '@/components/file/SharedPage.vue';
import ShareExportModal from '@/components/file/ShareExportModal.vue';
import TaskList from '@/components/task/TaskList.vue';
import NetworkPage from '@/components/network/NetworkPage.vue';
import WindowControls from '@/components/layout/WindowControls.vue';
import Toast from '@/components/ui/Toast.vue';
import { Plus, Download, Moon, Sun } from 'lucide-vue-next';
import api from '@/api';
import { useTaskStore } from '@/stores/tasks';
import { useAccountStore } from '@/stores/account';
import AccountSetup from '@/components/account/AccountSetup.vue';
import AccountUnlock from '@/components/account/AccountUnlock.vue';

import AccountProfile from '@/components/account/AccountProfile.vue';

const { t } = useI18n();
const fileStore = useFileStore();
const settingsStore = useSettingsStore();
const toastStore = useToastStore();
const taskStore = useTaskStore();
const networkStore = useNetworkStore();
const accountStore = useAccountStore();

const { files, loading, uploading } = storeToRefs(fileStore);

const currentTab = ref('files');
const showUploadModal = ref(false);
const showShareExportModal = ref(false);
const shareExportFile = ref<any>(null);
const isDark = ref(localStorage.getItem('theme') === 'dark');

const toggleTheme = () => {
  isDark.value = !isDark.value;
  if (isDark.value) {
    document.documentElement.classList.add('dark');
    localStorage.setItem('theme', 'dark');
  } else {
    document.documentElement.classList.remove('dark');
    localStorage.setItem('theme', 'light');
  }
};

// Initialize theme
onMounted(async () => {
  if (isDark.value) {
    document.documentElement.classList.add('dark');
  }
  
  await accountStore.checkStatus();
  
  if (accountStore.configured && !accountStore.locked) {
      fileStore.fetchFiles();
      settingsStore.fetchSettings();
  }
});

// Watch for unlock to load data
watch(() => accountStore.locked, (isLocked) => {
    if (!isLocked) {
        fileStore.fetchFiles();
        settingsStore.fetchSettings();
    }
});

// Watch tab change to auto-refresh/sync
watch(currentTab, async (newTab) => {
    if (newTab === 'files') {
        // 1. Fetch DB first to show local changes (like new Pins)
        await fileStore.fetchFiles();
        
        // 2. Auto-sync if online (Background)
        // We check networkStore status. 
        // Note: networkStore.status might be initial default, but it's reactive.
        // Better to check if we have a peerID or online flag.
        if (networkStore.status.online || networkStore.status.peer_id) {
             console.log("Auto-syncing files from node...");
             fileStore.syncFiles().catch(e => console.error("Auto-sync failed", e));
        }
    }
});

const handleUpload = async (data: any) => {
    try {
        await fileStore.uploadFile(data.file, data.options);
        showUploadModal.value = false;
        toastStore.success('File upload started in background');
    } catch (e) {
        toastStore.error('Upload failed to start');
    }
};

const handleDelete = async (id: number) => {
    if (confirm('Are you sure you want to delete this file?')) {
        await fileStore.deleteFile(id);
        toastStore.success('File deleted');
    }
};

const handleSync = async () => {
    try {
        await fileStore.syncFiles();
        toastStore.success('Files synced with IPFS node');
    } catch (e) {
        toastStore.error('Sync failed');
    }
};

const handlePreview = (file: any) => {
    let url = `http://localhost:3666/api/preview/${file.cid}`;
    window.open(url, '_blank');
};

const handleDownload = async (file: any) => {
    // Validate CID
    if (!file.cid) {
        toastStore.error("Error: File CID is missing. Please re-upload this file.");
        return;
    }

    // Determine where to save
    let downloadPath = '';
    
    // Check settings for "Always ask" or missing path
    if (settingsStore.askPath || !settingsStore.downloadPath) {
        // Since we are in browser environment, we can't easily open a native directory picker
        // and return the path to the code.
        // However, we can rely on the browser's default behavior which is:
        // 1. If user browser settings say "Ask where to save", it asks.
        // 2. Otherwise it saves to default Downloads.
        
        // But the requirement says: "If enabled... let user choose path".
        // In a pure web app, we can't force a path unless we use the File System Access API (showSaveFilePicker)
        // which returns a handle, not a path string we can pass to 'a' tag.
        
        // Strategy:
        // If "Ask Path" is enabled (or no path set), we rely on browser prompt (by not setting a specific behavior? No, browser controls this).
        // Actually, for Electron, we might be able to invoke a dialog via IPC? 
        // But here we are using standard web download flow.
        
        // Wait, for Electron apps, usually we can configure the 'will-download' event in main process.
        // But since we are doing client-side blob download:
        // window.showSaveFilePicker() is the modern way.
        
        try {
            // @ts-ignore
            if (window.showSaveFilePicker) {
                // @ts-ignore
                const handle = await window.showSaveFilePicker({
                    suggestedName: file.name
                });
                // If user picked a file, we get a writable stream
                // We can stream the download directly to it!
                
                const taskId = taskStore.addTask('download', file.name);
                toastStore.success(`Download started for ${file.name}`);
                
                try {
                    const writable = await handle.createWritable();
                    const baseUrl = api.defaults.baseURL || 'http://localhost:3666/api';
                    const url = `${baseUrl}/preview/${file.cid}?download=true`;
                    
                    // We need to fetch as stream and pipe to writable
                    const response = await fetch(url);
                    if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
                    if (!response.body) throw new Error("No response body");
                    
                    const total = Number(response.headers.get('content-length')) || 0;
                    let loaded = 0;
                    
                    const reader = response.body.getReader();
                    
                    // Pump
                    while (true) {
                        const { done, value } = await reader.read();
                        if (done) break;
                        loaded += value.length;
                        if (total) taskStore.updateProgress(taskId, loaded, total);
                        await writable.write(value);
                    }
                    
                    await writable.close();
                    taskStore.completeTask(taskId);
                    return; // Done
                    
                } catch (e: any) {
                    console.error(e);
                    taskStore.failTask(taskId, e.message);
                    toastStore.error(`Download failed: ${e.message}`);
                    return;
                }
            }
        } catch (err: any) {
            // User cancelled picker or not supported
            if (err.name === 'AbortError') return;
            console.warn("FileSystemAccess API not supported or failed, falling back to default download", err);
        }
    }

    // Fallback / Default Download Logic (Browser default or Settings Path if we could control it)
    // Note: In standard web, we CANNOT force a download path to 'settingsStore.downloadPath'.
    // The browser always saves to its configured Downloads folder.
    // The only way to save to a specific folder on Disk from Electron is to use the Main Process to handle download
    // OR use the FileSystem Access API (as above).
    
    // IF we want to strictly follow "Save to settings path", we MUST send a request to Backend to download to disk
    // (which we deprecated/disabled in favor of client-side tracking).
    
    // COMPROMISE:
    // 1. If "Ask Path" is ON -> Try showSaveFilePicker (Client side save).
    // 2. If "Ask Path" is OFF -> We want to save to `settings.downloadPath`.
    //    Client-side JS cannot write to arbitrary path `C:\MyFiles`.
    //    So we MUST use the Backend for this case IF we want to respect the path.
    //    BUT we want progress tracking...
    
    // SOLUTION:
    // Re-enable Backend Download for "Silent Save to Specific Path" scenario?
    // OR: Use Electron IPC to download?
    // Since we are in a rush and refactoring to Client Side download for Progress:
    // We will stick to Client Side Download (Blob/Anchor) which saves to Browser Default Downloads.
    // The "Settings Download Path" is effectively ignored in this mode unless we use Electron IPC.
    
    // However, the user asked to implement the logic.
    // Let's implement the "Ask Path" using showSaveFilePicker as it fits the "Client Side" model well.
    // For "Silent Save", we'll just do the Anchor click (Browser Default).
    
    // If settingsStore.downloadPath is set and "Ask" is false, we use the Backend to save silently
    if (!settingsStore.askPath && settingsStore.downloadPath) {
        const taskId = taskStore.addTask('download', file.name);
        toastStore.success(`Download started for ${file.name}`);
        
        try {
            // Use backend to download
            await api.post(`/files/${file.id}/download`);
            taskStore.completeTask(taskId);
        } catch (e: any) {
             console.error(e);
             taskStore.failTask(taskId, e.message || 'Download failed');
             toastStore.error(`Download failed: ${e.message}`);
        }
        return;
    }
    
    const taskId = taskStore.addTask('download', file.name);
    toastStore.success(`Download started for ${file.name}`);
    
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
        link.download = file.name;
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

const handleShare = (file: any) => {
    if (!file.cid) {
        toastStore.error("Error: File CID is missing. Please re-upload this file.");
        return;
    }
    shareExportFile.value = file;
    showShareExportModal.value = true;
};

const dataDirPath = ref('');

const handleUpdateDataDir = async () => {
    if (!dataDirPath.value) return;
    if (!confirm('This will move your data to the new location and restart the application. Are you sure?')) return;
    
    try {
        await settingsStore.setDataDir(dataDirPath.value);
        toastStore.success('Data moved. Application restarting...');
        // Wait for shutdown
        setTimeout(() => {
           // @ts-ignore
           window.electronAPI?.restart();
        }, 3000);
    } catch (e: any) {
        toastStore.error(e.response?.data?.error || 'Failed to move data');
    }
};
</script>

<template>
  <div class="h-screen w-screen bg-white dark:bg-nord-0 text-nord-0 dark:text-nord-6 font-sans flex overflow-hidden transition-colors duration-300">
    
    <Toast />

    <!-- Loading Splash -->
    <div v-if="accountStore.loading" class="fixed inset-0 bg-white dark:bg-nord-0 flex items-center justify-center z-[200]">
        <div class="flex flex-col items-center gap-4">
            <img src="/icon.png" class="w-16 h-16 animate-pulse" />
            <div class="animate-spin rounded-full h-8 w-8 border-4 border-nord-4 dark:border-nord-3 border-t-nord-10"></div>
        </div>
    </div>

    <!-- Account Setup Wizard -->
    <AccountSetup v-else-if="!accountStore.configured" />
    
    <!-- Unlock Screen -->
    <AccountUnlock v-else-if="accountStore.locked" />

    <!-- Main Content -->
    <template v-else>
    <!-- Sidebar -->
    <Sidebar v-model:currentTab="currentTab" />

    <!-- Main Content -->
    <div class="flex-1 flex flex-col min-w-0">
      
      <!-- Header -->
      <header class="h-16 border-b border-nord-4 dark:border-nord-2 flex justify-between items-center px-6 draggable bg-white dark:bg-nord-0 select-none">
        <div class="no-drag flex items-center gap-4">
           <h2 class="text-xl font-semibold capitalize">{{ currentTab.replace('-', ' ') }}</h2>
        </div>
        <div class="no-drag flex items-center gap-4">
            <button 
                @click="showUploadModal = true"
                class="flex items-center gap-2 bg-nord-10 hover:bg-nord-9 text-white px-4 py-2 rounded-lg font-medium transition-colors shadow-sm"
            >
                <Plus class="w-4 h-4" /> Upload
            </button>
            <div class="h-6 w-px bg-nord-4 dark:bg-nord-2 mx-2"></div>
            <WindowControls />
        </div>
      </header>

      <!-- Content Body -->
      <main class="flex-1 overflow-auto p-6 relative">
        
        <!-- File List -->
        <div v-if="currentTab === 'files'" class="animate-fade-in">
            <div v-if="loading" class="flex justify-center py-12">
                <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-nord-10"></div>
            </div>
            <FileTable 
                v-else 
                :files="files" 
                :show-sync="true"
                @preview="handlePreview"
                @share="handleShare"
                @download="handleDownload"
                @delete="handleDelete"
                @sync="handleSync"
            />
        </div>

        <!-- Shared Tab -->
        <div v-else-if="currentTab === 'shared'" class="h-full animate-fade-in">
             <SharedPage />
        </div>

        <!-- Tasks Tab -->
        <div v-else-if="currentTab === 'tasks'" class="h-full animate-fade-in">
             <TaskList />
        </div>

        <!-- Network Tab -->
        <div v-else-if="currentTab === 'network'" class="h-full animate-fade-in">
             <NetworkPage />
        </div>

        <!-- Account Tab -->
        <div v-else-if="currentTab === 'account'" class="h-full animate-fade-in">
             <AccountProfile />
        </div>

        <!-- Settings Tab -->
        <div v-else-if="currentTab === 'settings'" class="p-8 max-w-2xl mx-auto space-y-8 animate-fade-in">
             <div class="space-y-4">
                 <h3 class="text-lg font-bold text-nord-1 dark:text-nord-6">Appearance</h3>
                 <div class="bg-nord-6 dark:bg-nord-1 p-4 rounded-xl border border-nord-4 dark:border-nord-2 flex items-center justify-between">
                     <div>
                         <label class="text-sm font-bold text-nord-1 dark:text-nord-5">Theme Mode</label>
                         <p class="text-xs text-nord-3 mt-1">Switch between light and dark themes.</p>
                     </div>
                     <button @click="toggleTheme" class="p-2 rounded-lg bg-white dark:bg-nord-0 border border-nord-4 dark:border-nord-3 hover:bg-nord-4 dark:hover:bg-nord-2 transition-colors">
                         <Moon v-if="isDark" class="w-5 h-5 text-nord-10" />
                         <Sun v-else class="w-5 h-5 text-orange-400" />
                     </button>
                 </div>
             </div>
             
             <div class="space-y-4">
                 <h3 class="text-lg font-bold text-nord-1 dark:text-nord-6">Download Settings</h3>
                 <div class="bg-nord-6 dark:bg-nord-1 p-4 rounded-xl border border-nord-4 dark:border-nord-2 space-y-4">
                     <div>
                         <label class="text-xs font-bold text-nord-3 dark:text-nord-4 uppercase">Download Path</label>
                         <div class="flex gap-2 mt-1">
                             <input 
                                v-model="settingsStore.downloadPath" 
                                type="text" 
                                placeholder="Absolute path (e.g. C:\Users\Name\Downloads)"
                                class="flex-1 px-3 py-2 rounded-lg border border-nord-4 dark:border-nord-3 bg-white dark:bg-nord-0 text-sm font-mono"
                             />
                             <button @click="settingsStore.updateSettings(settingsStore.downloadPath, settingsStore.askPath, settingsStore.ipfsApiUrl, settingsStore.useEmbeddedNode)" class="px-4 py-2 bg-nord-10 text-white rounded-lg text-sm font-medium hover:bg-nord-9">Save</button>
                         </div>
                         <p class="text-xs text-nord-3 mt-1">Files will be saved to this folder on your computer.</p>
                     </div>

                     <div class="flex items-center gap-3">
                         <input 
                            type="checkbox" 
                            id="askPath"
                            v-model="settingsStore.askPath"
                            @change="settingsStore.updateSettings(settingsStore.downloadPath, settingsStore.askPath, settingsStore.ipfsApiUrl)"
                            class="w-4 h-4 rounded border-nord-4 dark:border-nord-3 text-nord-10 focus:ring-nord-10"
                         />
                         <label for="askPath" class="text-sm text-nord-1 dark:text-nord-5 select-none">Always ask where to save files</label>
                     </div>
                 </div>
            </div>

             <div class="space-y-4">
                 <h3 class="text-lg font-bold text-nord-1 dark:text-nord-6">Data Storage</h3>
                 <div class="bg-nord-6 dark:bg-nord-1 p-4 rounded-xl border border-nord-4 dark:border-nord-2 space-y-4">
                     <div>
                         <label class="text-xs font-bold text-nord-3 dark:text-nord-4 uppercase">Storage Location</label>
                         <div class="flex gap-2 mt-1">
                             <input 
                                v-model="dataDirPath" 
                                type="text" 
                                :placeholder="networkStore.status.data_dir || 'Enter absolute path (e.g. D:\\MochiData)'"
                                class="flex-1 px-3 py-2 rounded-lg border border-nord-4 dark:border-nord-3 bg-white dark:bg-nord-0 text-sm font-mono"
                             />
                             <button @click="handleUpdateDataDir" class="px-4 py-2 bg-nord-10 text-white rounded-lg text-sm font-medium hover:bg-nord-9 whitespace-nowrap">Move & Restart</button>
                         </div>
                         <p class="text-xs text-nord-3 mt-1">
                            Current: <span class="font-mono bg-nord-5 dark:bg-nord-2 px-1 rounded">{{ networkStore.status.data_dir }}</span>
                            <br/>
                            Move your database and IPFS repository to a new location. <span class="text-red-500">App will restart.</span>
                         </p>
                     </div>
                 </div>
            </div>

            <!-- IPFS Connection Settings -->
            <div class="space-y-4">
                 <h3 class="text-lg font-bold text-nord-1 dark:text-nord-6">IPFS Node Connection</h3>
                 <div class="bg-nord-6 dark:bg-nord-1 p-4 rounded-xl border border-nord-4 dark:border-nord-2 space-y-4">
                     
                     <!-- Embedded Node Toggle -->
                     <div class="flex items-center gap-4 pb-4 border-b border-nord-4 dark:border-nord-2">
                         <button 
                            @click="settingsStore.updateSettings(settingsStore.downloadPath, settingsStore.askPath, settingsStore.ipfsApiUrl, !settingsStore.useEmbeddedNode)"
                            class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-nord-10 focus:ring-offset-2"
                            :class="settingsStore.useEmbeddedNode ? 'bg-nord-10' : 'bg-nord-4 dark:bg-nord-3'"
                            role="switch"
                            :aria-checked="settingsStore.useEmbeddedNode"
                         >
                            <span class="sr-only">Use Built-in IPFS Node</span>
                            <span 
                                aria-hidden="true" 
                                class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                                :class="settingsStore.useEmbeddedNode ? 'translate-x-5' : 'translate-x-0'"
                            ></span>
                         </button>
                         <div>
                             <span class="text-sm font-bold text-nord-1 dark:text-nord-5 select-none block">Use Built-in IPFS Node (Recommended)</span>
                             <p class="text-xs text-nord-3 flex items-center gap-2">
                                Automatically manages a local IPFS node for you.
                                <span v-if="networkStore.isStarting" class="text-amber-500 font-medium flex items-center gap-1">
                                    <span class="w-2 h-2 border-2 border-amber-500 border-t-transparent rounded-full animate-spin"></span>
                                    Starting...
                                </span>
                             </p>
                         </div>
                     </div>

                     <div>
                         <label class="text-xs font-bold text-nord-3 dark:text-nord-4 uppercase">API Address</label>
                         <div class="flex gap-2 mt-1">
                             <input 
                                v-model="settingsStore.ipfsApiUrl" 
                                type="text" 
                                :disabled="settingsStore.useEmbeddedNode"
                                placeholder="http://127.0.0.1:5001"
                                class="flex-1 px-3 py-2 rounded-lg border border-nord-4 dark:border-nord-3 bg-white dark:bg-nord-0 text-sm font-mono disabled:opacity-50 disabled:bg-nord-5"
                             />
                             <button 
                                v-if="!settingsStore.useEmbeddedNode"
                                @click="settingsStore.updateSettings(settingsStore.downloadPath, settingsStore.askPath, settingsStore.ipfsApiUrl, settingsStore.useEmbeddedNode)" 
                                class="px-4 py-2 bg-nord-10 text-white rounded-lg text-sm font-medium hover:bg-nord-9"
                             >
                                Connect
                             </button>
                         </div>
                         <p v-if="settingsStore.useEmbeddedNode" class="text-xs text-nord-10 mt-1 font-medium">Managed automatically by MochiBox.</p>
                         <p v-else class="text-xs text-nord-3 mt-1">URL of the external IPFS node API you want to control.</p>
                     </div>
                 </div>
            </div>
        </div>

        <!-- Placeholder for other tabs -->
        <div v-else class="h-full flex items-center justify-center text-nord-3 dark:text-nord-4">
            Coming Soon...
        </div>

      </main>
    </div>
    </template>

    <!-- Modals -->
    <UploadModal 
        :is-open="showUploadModal" 
        @close="showUploadModal = false"
        @upload="handleUpload"
    />

    <ShareExportModal
        :is-open="showShareExportModal"
        :file="shareExportFile"
        @close="showShareExportModal = false"
    />

    <!-- Uploading Overlay -->
    <div v-if="uploading" class="fixed inset-0 z-[60] bg-black/50 backdrop-blur-sm flex items-center justify-center">
        <div class="bg-white dark:bg-nord-1 p-8 rounded-2xl shadow-2xl flex flex-col items-center gap-4">
            <div class="animate-spin rounded-full h-12 w-12 border-4 border-nord-4 dark:border-nord-3 border-t-nord-10"></div>
            <p class="font-medium">Uploading to IPFS...</p>
        </div>
    </div>

  </div>
</template>

<style>
.draggable {
  -webkit-app-region: drag;
}
.no-drag {
  -webkit-app-region: no-drag;
}
.animate-fade-in {
  animation: fadeIn 0.3s ease-out;
}
@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>