<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { Download, Trash2, History } from 'lucide-vue-next';
import { useSharedStore } from '@/stores/shared';
import { useToastStore } from '@/stores/toast';
import { useTaskStore } from '@/stores/tasks';
import { useSettingsStore } from '@/stores/settings';
import { useAccountStore } from '@/stores/account';
import { useNetworkStore } from '@/stores/network';
import { storeToRefs } from 'pinia';
import FileTable from '@/components/file/FileTable.vue';
import SharedFileModal from '@/components/file/SharedFileModal.vue';
import FolderPreviewModal from '@/components/file/FolderPreviewModal.vue';
import FilePreviewModal from '@/components/file/FilePreviewModal.vue';
import api from '@/api';

const sharedStore = useSharedStore();
const toastStore = useToastStore();
const taskStore = useTaskStore();
const settingsStore = useSettingsStore();
const accountStore = useAccountStore();
const networkStore = useNetworkStore();

const { history, loading } = storeToRefs(sharedStore);
const sharedInput = ref('');
const showSharedModal = ref(false);
const sharedModalData = ref<any>(null);

const showFolderModal = ref(false);
const selectedFolder = ref<any>(null);

const showPreviewModal = ref(false);
const previewUrl = ref('');
const previewName = ref('');
const previewMime = ref('');

// Search State
const searchStatus = ref('idle');
const peersCount = ref(0);
let searchEventSource: EventSource | null = null;

onMounted(() => {
    sharedStore.fetchHistory();
    
    // Auto-boost network if peers count is low (e.g. < 20)
    // This helps when the node has been idle for a long time
    if (networkStore.status.peers < 20) {
        const toastId = toastStore.info('Boost Network Started');
        api.post('/system/bootstrap')
            .then(() => {
                toastStore.success('Boost Network Finished');
            })
            .catch((e: any) => {
                if (e.response && e.response.status === 409) {
                    toastStore.remove(toastId);
                    toastStore.warning('Boost Network is running');
                } else {
                    // Ignore other errors for auto-boost to avoid spamming error toasts
                    console.warn('Auto-boost failed', e);
                }
            });
    }
});

const handleImportShared = async () => {
    try {
        const input = sharedInput.value.trim();
        let cid = '';
        let name = '';
        let size = 0;
        let mimeType = '';
        let encryptionType = 'public';
        let encryptionMeta = '';
        let embeddedPassword = '';
        let peers: string[] = [];

        // Reset Search State
        searchStatus.value = 'idle';
        peersCount.value = 0;
        if (searchEventSource) {
            searchEventSource.close();
            searchEventSource = null;
        }

        // 1. Try Parse Mochi Link (mochi://BASE64)
        if (input.startsWith('mochi://')) {
            try {
                const base64Str = input.replace('mochi://', '');
                
                // UTF-8 Safe Base64 Decoding
                const binaryStr = atob(base64Str);
                const bytes = new Uint8Array(binaryStr.length);
                for (let i = 0; i < binaryStr.length; i++) {
                    bytes[i] = binaryStr.charCodeAt(i);
                }
                
                const decoder = new TextDecoder();
                const jsonStr = decoder.decode(bytes);
                
                const payload = JSON.parse(jsonStr);
                
                cid = payload.c;
                name = payload.n || '';
                size = payload.s || 0;
                mimeType = payload.m || '';
                if (payload.peers) peers = payload.peers;
                
                // Map short type to full type
                if (payload.t === 'pwd') encryptionType = 'password';
                else if (payload.t === 'priv') encryptionType = 'private';
                else encryptionType = 'public';
                
                // Extract Meta
                if (payload.p) {
                    if (encryptionType === 'password') {
                        encryptionMeta = payload.p.salt;
                        embeddedPassword = payload.p.pw || '';
                    }
                    else if (encryptionType === 'private') encryptionMeta = payload.p.ek;
                }
            } catch (e) {
                throw new Error("Invalid Mochi Link format");
            }
        } else {
            // 2. Try JSON Parse
            try {
                const data = JSON.parse(input);
                if (data.cid) {
                    cid = data.cid;
                    name = data.name || '';
                } else {
                    throw new Error("Invalid JSON");
                }
            } catch {
                // 3. Treat as raw CID/String
                if (input.length > 5 && !input.includes(' ')) {
                    cid = input;
                } else if (input.startsWith('http')) {
                     window.open(input, '_blank');
                     return;
                } else {
                     throw new Error("Invalid format");
                }
            }
        }

        if (!cid) throw new Error("Invalid share data: missing CID");

        if (!name) {
            name = `Shared-${cid.slice(0, 8)}`;
        }

        const modalFile: any = {
            cid,
            name,
            size,
            mime_type: mimeType,
            encryption_type: encryptionType,
            encryption_meta: encryptionMeta
        };
        if (embeddedPassword) {
            modalFile.embedded_password = embeddedPassword;
        }

        sharedModalData.value = modalFile;
        showSharedModal.value = true;
        
        sharedInput.value = '';

        // Start Search & Connect
        if (peers.length > 0) {
            api.post('/shared/connect', { peers }).catch(console.warn);
        }
        
        searchStatus.value = 'searching';
        const baseUrl = api.defaults.baseURL || 'http://localhost:3666/api';
        
        // Ensure we close previous
        if (searchEventSource) searchEventSource.close();
        
        searchEventSource = new EventSource(`${baseUrl}/shared/search/${cid}`);
        
        searchEventSource.addEventListener('update', (e) => {
             try {
                const data = JSON.parse(e.data);
                peersCount.value = data.peers;
                if (data.peers > 0) searchStatus.value = 'found';
             } catch {}
        });
        
        searchEventSource.addEventListener('done', (e) => {
            searchEventSource?.close();
            if (peersCount.value === 0) searchStatus.value = 'idle'; // Not found
            else searchStatus.value = 'found';
        });
        
        searchEventSource.onerror = () => {
            searchEventSource?.close();
            if (peersCount.value === 0) searchStatus.value = 'idle';
        };

        sharedStore.addToHistory(cid, name, size, mimeType, encryptionType, encryptionMeta, input)
            .then((saved) => {
                saved.encryption_type = encryptionType;
                saved.encryption_meta = encryptionMeta;
                if (embeddedPassword) saved.embedded_password = embeddedPassword;
                if (sharedModalData.value?.cid === cid) {
                    sharedModalData.value = { ...sharedModalData.value, ...saved };
                }
                toastStore.success('Added to history');
            })
            .catch((e: any) => {
                toastStore.error(e?.response?.data?.error || e?.message || 'Failed to add to history');
            });
        
    } catch (e: any) {
         toastStore.error(e.message || 'Invalid share format');
    }
};

const handlePreview = (file: any, password?: string) => {
    // Check if folder
    if (file.mime_type === 'inode/directory' || file.is_folder) {
        selectedFolder.value = file;
        showFolderModal.value = true;
        return;
    }

    const baseUrl = api.defaults.baseURL || 'http://localhost:3666/api';
    let url = `${baseUrl}/preview/${file.cid}`;
    const params = new URLSearchParams();

    // Append encryption params if known
    if (file.encryption_type === 'password') {
        // Fix: Check file.password (from FileTable emit) as well
        let pw = password || file.password || '';
        if (file.embedded_password) {
            pw = file.embedded_password;
        }
        
        if (!pw) {
            sharedModalData.value = file;
            showSharedModal.value = true;
            return;
        }
        params.append('password', pw);
        params.append('type', 'password');
        
        // Stateless fallback
        if (file.encryption_meta) {
            params.append('meta', file.encryption_meta);
        }
        
    } else if (file.encryption_type === 'private') {
        if (accountStore.locked) {
            toastStore.error("Please unlock your account first");
            return;
        }
        params.append('type', 'private');
        // Stateless fallback
        if (file.encryption_meta) {
            params.append('meta', file.encryption_meta);
        }
    }
    
    if (params.toString()) {
        url += `?${params.toString()}`;
    }
    
    // Check for In-App Preview (Images / Text / PDF)
    const mime = file.mime_type || '';
    if (mime.startsWith('image/') || mime.startsWith('text/') || mime === 'application/pdf') {
        previewUrl.value = url;
        previewName.value = file.name;
        previewMime.value = mime;
        showPreviewModal.value = true;
    } else {
        window.open(url, '_blank');
    }
};

const handleDownload = async (file: any, password?: string) => {
    const filename = file.name || file.cid;
    
    // Resolve password first
    let pw = password || '';
    if (file.encryption_type === 'password') {
        if (file.embedded_password) {
            pw = file.embedded_password;
        }
        if (!pw) {
            sharedModalData.value = file;
            showSharedModal.value = true;
            return;
        }
    } else if (file.encryption_type === 'private' && accountStore.locked) {
         toastStore.error("Please unlock your account first");
         return;
    }

    // Determine Strategy
    // Strategy A: Frontend FileSystem API / Blob (Interactive or Fallback)
    // Strategy B: Backend Silent Download (Only if path set, ask disabled, and file not encrypted)
    
    // Note: Backend currently does not support stateless encrypted download easily.
    // So we force Encrypted files to use Frontend Download for now.
    const isEncrypted = file.encryption_type === 'password' || file.encryption_type === 'private';
    const useBackendSilent = !settingsStore.askPath && settingsStore.downloadPath && !isEncrypted;

    const taskId = taskStore.addTask('download', filename);
    toastStore.success(`Download started for ${filename}`);

    try {
        if (useBackendSilent) {
             // Strategy B: Backend Silent
             await api.post('/files/download/shared', {
                cid: file.cid,
                name: filename,
            });
            taskStore.completeTask(taskId);
            return;
        }
        
        // Strategy A: Frontend Download
        // Try File System Access API first if "Ask Path" is enabled
        if (settingsStore.askPath) {
             try {
                // @ts-ignore
                if (window.showSaveFilePicker) {
                    // @ts-ignore
                    const handle = await window.showSaveFilePicker({
                        suggestedName: filename
                    });
                    
                    const writable = await handle.createWritable();
                    const baseUrl = api.defaults.baseURL || 'http://localhost:3666/api';
                    let url = `${baseUrl}/preview/${file.cid}?download=true`;
                    
                    const params = new URLSearchParams();
                    if (pw) params.append('password', pw);
                    if (file.encryption_meta) params.append('meta', file.encryption_meta);
                    if (file.encryption_type) params.append('type', file.encryption_type);
                    if (params.toString()) url += `&${params.toString()}`;
                    
                    const response = await fetch(url);
                    if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
                    
                    // @ts-ignore
                    await response.body.pipeTo(writable);
                    
                    taskStore.completeTask(taskId);
                    return;
                }
             } catch (err: any) {
                 if (err.name === 'AbortError') {
                     taskStore.removeTask(taskId); // Cancelled
                     return;
                 }
                 console.warn("FileSystemAccess API failed, falling back to Blob", err);
                 // Fallthrough to Blob
             }
        }
        
        // Fallback: Blob Download (Browser Default)
        let url = `/preview/${file.cid}?download=true`;
        const params = new URLSearchParams();
        if (pw) params.append('password', pw);
        if (file.encryption_meta) params.append('meta', file.encryption_meta);
        if (file.encryption_type) params.append('type', file.encryption_type);
        if (params.toString()) url += `&${params.toString()}`;

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
        console.error(e);
        taskStore.failTask(taskId, e.message || 'Download failed');
        toastStore.error(`Download failed: ${e.message}`);
    }
};

const handleShare = (file: any) => {
    // Prefer original link if available
    if (file.original_link) {
        navigator.clipboard.writeText(file.original_link);
        toastStore.success('Mochi Link copied to clipboard');
        return;
    }

    // Copy CID to clipboard
    navigator.clipboard.writeText(file.cid);
    toastStore.success('CID copied to clipboard (No Link Metadata)');
};

const handlePin = async (file: any) => {
    const filename = file.name || file.cid;
    const taskId = taskStore.addTask('download', `Pinning ${filename}...`); // Use download type for icon/progress semantics if needed, or generic
    toastStore.success(`Pinning started for ${filename}`);
    
    try {
        await api.post('/shared/pin', { 
            cid: file.cid,
            encryption_type: file.encryption_type,
            encryption_meta: file.encryption_meta
        });
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

const onModalPreview = (password?: string) => {
    if (sharedModalData.value) {
        handlePreview(sharedModalData.value, password);
    }
};

const onModalDownload = (password?: string) => {
    if (sharedModalData.value) {
        handleDownload(sharedModalData.value, password);
        // Only close if we are not prompting for password inside?
        // Wait, handleDownload might re-open modal if password missing.
        // But if we pass password, it should succeed or fail.
        // Let's close modal only if we have password or not password type.
        
        // Actually, handleDownload is async.
        // If password provided, we can close modal.
        if (sharedModalData.value.encryption_type !== 'password' || password || sharedModalData.value.embedded_password) {
             showSharedModal.value = false;
        }
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
            :search-status="searchStatus"
            :peers-count="peersCount"
            @close="showSharedModal = false"
            @preview="onModalPreview"
            @download="onModalDownload"
            @pin="onModalPin"
        />

        <FolderPreviewModal
            v-if="selectedFolder"
            :is-open="showFolderModal"
            :cid="selectedFolder.cid"
            :name="selectedFolder.name"
            @close="showFolderModal = false"
            @preview="handlePreview"
        />

        <FilePreviewModal
            :is-open="showPreviewModal"
            :url="previewUrl"
            :name="previewName"
            :mime-type="previewMime"
            @close="showPreviewModal = false"
        />

    </div>
</template>
