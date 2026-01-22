<script setup lang="ts">
import { Download, Eye, FileText, X, Pin, Lock, Globe, UserCheck, Copy, CheckCircle } from 'lucide-vue-next';
import { ref, watch, computed } from 'vue';
import { useToastStore } from '@/stores/toast';
import { copyToClipboard } from '@/utils/clipboard';

const props = defineProps<{
  isOpen: boolean;
  sharedData: any;
  searchStatus?: string; // 'idle' | 'searching' | 'found'
  peersCount?: number;
  connectStatus?: string; // 'idle' | 'connecting' | 'done'
  connectResult?: any;
  cidVerified?: boolean;
  cidSize?: number;
}>();

const emit = defineEmits(['close', 'preview', 'download', 'pin']);
const toastStore = useToastStore();
const passwordInput = ref('');

// Reset password when modal opens
watch(() => props.isOpen, (newVal) => {
    if (newVal) passwordInput.value = '';
});

const isSearching = computed(() => props.searchStatus === 'searching');
const isConnecting = computed(() => props.connectStatus === 'connecting');

// File is ready when: CID is verified OR we found peers via DHT search
const isReady = computed(() => {
    return props.cidVerified || (props.peersCount || 0) > 0;
});

// Still loading if: searching DHT OR connecting to peers (and not yet verified)
const isLoading = computed(() => {
    return (isSearching.value || isConnecting.value) && !isReady.value;
});

// Display size - prefer verified size, fallback to sharedData size
const displaySize = computed(() => {
    if (props.cidSize && props.cidSize > 0) return props.cidSize;
    return props.sharedData?.size || 0;
});

const handleAction = (action: 'preview' | 'download') => {
    emit(action, passwordInput.value);
};

const copyCID = async () => {
    if (props.sharedData?.cid) {
        const success = await copyToClipboard(props.sharedData.cid);
        if (success) {
            toastStore.success('CID copied');
        } else {
            toastStore.error('Failed to copy');
        }
    }
};

const copyConnectReport = async () => {
    const report = props.connectResult ? JSON.stringify(props.connectResult, null, 2) : '';
    if (!report) return;
    
    const success = await copyToClipboard(report);
    if (success) {
        toastStore.success('Connect report copied');
    } else {
        toastStore.error('Failed to copy');
    }
};

const formatSize = (bytes: number) => {
    if (!bytes || bytes === 0) return 'Unknown';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
};
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
        <div class="flex items-start gap-4 p-4 bg-nord-6 dark:bg-nord-2 rounded-xl relative overflow-hidden">
            <div class="p-3 bg-white dark:bg-nord-3 rounded-lg shadow-sm">
                <FileText class="w-8 h-8 text-nord-10" />
            </div>
            <div class="flex-1 min-w-0">
                <h4 class="font-bold text-nord-1 dark:text-nord-6 truncate">{{ sharedData?.name || 'Unknown Filename' }}</h4>
                
                <div class="flex items-center gap-2 group cursor-pointer" @click="copyCID" title="Click to copy CID">
                    <p class="text-xs font-mono text-nord-3 dark:text-nord-4 truncate max-w-[200px] hover:text-nord-10 transition-colors">CID: {{ sharedData?.cid }}</p>
                    <Copy class="w-3 h-3 text-nord-3 opacity-0 group-hover:opacity-100 transition-opacity" />
                </div>
                
                <div class="flex items-center gap-2 mt-2">
                    <!-- Size Badge -->
                    <span class="text-xs font-mono text-nord-3 dark:text-nord-4 bg-white dark:bg-nord-3 px-2 py-0.5 rounded border border-nord-4 dark:border-nord-2">
                        {{ formatSize(displaySize) }}
                    </span>
                    
                    <!-- Encryption Badge -->
                    <div v-if="sharedData?.encryption_type === 'password'" class="flex items-center gap-1 px-2 py-0.5 bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-500 rounded text-xs font-bold uppercase border border-amber-200 dark:border-amber-900/50">
                        <Lock class="w-3 h-3" /> Password
                    </div>
                    <div v-else-if="sharedData?.encryption_type === 'private'" class="flex items-center gap-1 px-2 py-0.5 bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-500 rounded text-xs font-bold uppercase border border-purple-200 dark:border-purple-900/50">
                        <UserCheck class="w-3 h-3" /> Private
                    </div>
                    <div v-else class="flex items-center gap-1 px-2 py-0.5 bg-nord-4 dark:bg-nord-3 text-nord-2 dark:text-nord-4 rounded text-xs font-bold uppercase border border-nord-5 dark:border-nord-2">
                        <Globe class="w-3 h-3" /> Public
                    </div>
                </div>
            </div>
        </div>

        <!-- Password Input -->
        <div v-if="sharedData?.encryption_type === 'password' && !sharedData?.embedded_password" class="space-y-2 animate-fade-in">
            <label class="text-xs font-bold text-nord-3 dark:text-nord-4 uppercase">Password Required</label>
            <input 
                v-model="passwordInput"
                type="password" 
                placeholder="Enter password to decrypt..." 
                class="w-full px-4 py-2.5 rounded-xl border border-nord-4 dark:border-nord-3 bg-white dark:bg-nord-0 text-sm focus:ring-2 focus:ring-nord-10 outline-none transition-all"
                @keyup.enter="handleAction('preview')"
            />
        </div>
        
        <div v-if="connectStatus && connectStatus !== 'idle'" class="flex items-center justify-center gap-2 p-3 bg-nord-6 dark:bg-nord-2 rounded-xl text-sm transition-all duration-300">
             <div v-if="connectStatus === 'connecting'" class="w-2 h-2 rounded-full bg-nord-10 animate-ping"></div>
             <span v-if="connectStatus === 'connecting'" class="text-nord-3 dark:text-nord-4 font-medium">Connecting to peers...</span>
             <template v-else-if="connectResult?.status === 'done'">
                <span class="text-nord-3 dark:text-nord-4 font-medium">
                    Direct connect {{ connectResult.connected }} / {{ connectResult.attempted }}
                </span>
                <span v-if="cidVerified" class="text-green-600 dark:text-green-400 font-bold flex items-center gap-1 ml-2">
                    <CheckCircle class="w-4 h-4" /> Verified
                </span>
             </template>
             <span v-else-if="connectResult?.status === 'error'" class="text-red-500 font-medium">Direct connect failed</span>
             <button v-if="connectResult" @click="copyConnectReport" class="ml-2 p-1 text-nord-3 dark:text-nord-4 hover:text-nord-10 transition-colors" title="Copy connect report">
                <Copy class="w-4 h-4" />
             </button>
        </div>

        <!-- Search Status -->
        <div v-if="searchStatus && searchStatus !== 'idle' && !cidVerified" class="flex items-center justify-center gap-2 p-3 bg-nord-6 dark:bg-nord-2 rounded-xl text-sm transition-all duration-300">
             <div v-if="searchStatus === 'searching'" class="w-2 h-2 rounded-full bg-nord-10 animate-ping"></div>
             <span v-if="searchStatus === 'searching'" class="text-nord-3 dark:text-nord-4 font-medium">Searching DHT network...</span>
             <span v-else-if="searchStatus === 'found'" class="text-green-600 dark:text-green-400 font-bold flex items-center gap-2">
                 <div class="w-2 h-2 rounded-full bg-green-500"></div>
                 Found {{ peersCount }} Peers
             </span>
        </div>

        <!-- Actions -->
        <div class="flex gap-3">
            <button 
                @click="handleAction('preview')"
                :disabled="isLoading"
                :class="{'opacity-50 cursor-not-allowed': isLoading}"
                class="flex-1 py-3 px-4 bg-white dark:bg-nord-3 border border-nord-4 dark:border-nord-2 hover:bg-nord-5 dark:hover:bg-nord-2 text-nord-1 dark:text-nord-6 font-medium rounded-xl transition-colors flex items-center justify-center gap-2"
            >
                <Eye class="w-4 h-4" /> Preview
            </button>
            <button 
                @click="handleAction('download')"
                :disabled="isLoading"
                :class="{'opacity-50 cursor-not-allowed': isLoading}"
                class="flex-1 py-3 px-4 bg-nord-10 hover:bg-nord-9 text-white font-medium rounded-xl transition-colors shadow-lg shadow-nord-10/20 flex items-center justify-center gap-2"
            >
                <Download class="w-4 h-4" /> Download
            </button>
            <button 
                @click="$emit('pin')"
                :disabled="isLoading"
                :class="{'opacity-50 cursor-not-allowed': isLoading}"
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
