<script setup lang="ts">
import { Download, Eye, FileText, X, Pin, Lock, Globe, UserCheck, Copy } from 'lucide-vue-next';
import { ref, watch, computed } from 'vue';
import { useToastStore } from '@/stores/toast';

const props = defineProps<{
  isOpen: boolean;
  sharedData: any;
  searchStatus?: string; // 'idle' | 'searching' | 'found'
  peersCount?: number;
}>();

const emit = defineEmits(['close', 'preview', 'download', 'pin']);
const toastStore = useToastStore();
const passwordInput = ref('');

// Reset password when modal opens
watch(() => props.isOpen, (newVal) => {
    if (newVal) passwordInput.value = '';
});

const isSearching = computed(() => props.searchStatus === 'searching');
const hasPeers = computed(() => (props.peersCount || 0) > 0);

const handleAction = (action: 'preview' | 'download') => {
    emit(action, passwordInput.value);
};

const copyCID = async () => {
    if (props.sharedData?.cid) {
        try {
            await navigator.clipboard.writeText(props.sharedData.cid);
            toastStore.success('CID copied');
        } catch {
            toastStore.error('Failed to copy');
        }
    }
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
                    <span v-if="sharedData?.size !== undefined" class="text-xs font-mono text-nord-3 dark:text-nord-4 bg-white dark:bg-nord-3 px-2 py-0.5 rounded border border-nord-4 dark:border-nord-2">
                        {{ sharedData.size === 0 ? 'Unknown' : sharedData.size }}
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
        
        <!-- Search Status -->
        <div v-if="searchStatus && searchStatus !== 'idle'" class="flex items-center justify-center gap-2 p-3 bg-nord-6 dark:bg-nord-2 rounded-xl text-sm transition-all duration-300">
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
                :disabled="isSearching && !hasPeers"
                :class="{'opacity-50 cursor-not-allowed': isSearching && !hasPeers}"
                class="flex-1 py-3 px-4 bg-white dark:bg-nord-3 border border-nord-4 dark:border-nord-2 hover:bg-nord-5 dark:hover:bg-nord-2 text-nord-1 dark:text-nord-6 font-medium rounded-xl transition-colors flex items-center justify-center gap-2"
            >
                <Eye class="w-4 h-4" /> Preview
            </button>
            <button 
                @click="handleAction('download')"
                :disabled="isSearching && !hasPeers"
                :class="{'opacity-50 cursor-not-allowed': isSearching && !hasPeers}"
                class="flex-1 py-3 px-4 bg-nord-10 hover:bg-nord-9 text-white font-medium rounded-xl transition-colors shadow-lg shadow-nord-10/20 flex items-center justify-center gap-2"
            >
                <Download class="w-4 h-4" /> Download
            </button>
            <button 
                @click="$emit('pin')"
                :disabled="isSearching && !hasPeers"
                :class="{'opacity-50 cursor-not-allowed': isSearching && !hasPeers}"
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
