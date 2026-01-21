<script setup lang="ts">
import { Activity, Copy } from 'lucide-vue-next';
import { copyToClipboard } from '@/utils/clipboard';
import { useToastStore } from '@/stores/toast';

const props = defineProps<{
  isOpen: boolean;
  info: any;
}>();

const emit = defineEmits(['close']);
const toastStore = useToastStore();

const close = () => {
  emit('close');
};

const copyText = async (text: string) => {
  if (text) {
    const success = await copyToClipboard(text);
    if (success) {
      toastStore.success('Copied to clipboard');
    } else {
      toastStore.error('Failed to copy to clipboard');
    }
  }
};
</script>

<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" @click="close">
    <div class="bg-white dark:bg-nord-3 rounded-lg shadow-xl w-full max-w-lg p-6 relative border border-nord-4 dark:border-nord-2" @click.stop>
      <h3 class="text-xl font-bold text-nord-0 dark:text-nord-6 mb-4 flex items-center gap-2">
        <Activity class="w-5 h-5" />
        IPFS Node Info
      </h3>
      
      <div class="space-y-4">
        <!-- Status -->
        <div class="flex items-center justify-between p-3 bg-nord-6 dark:bg-nord-1 rounded-md">
          <span class="text-sm font-medium text-nord-3 dark:text-nord-4">Status</span>
          <div class="flex items-center gap-2">
            <span :class="info.online ? 'bg-green-500' : 'bg-red-500'" class="w-2.5 h-2.5 rounded-full animate-pulse"></span>
            <span class="text-sm font-bold" :class="info.online ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'">
              {{ info.online ? 'Online' : 'Offline' }}
            </span>
          </div>
        </div>

        <!-- Peer ID -->
        <div>
          <label class="block text-xs uppercase text-nord-3 dark:text-nord-4 font-bold mb-1">Peer ID</label>
          <div class="flex items-center gap-2">
            <code class="flex-1 bg-nord-5 dark:bg-nord-0 p-2 rounded text-xs font-mono break-all text-nord-1 dark:text-nord-5 border border-nord-4 dark:border-nord-1">
              {{ info.peer_id || 'Generating...' }}
            </code>
            <button @click="copyText(info.peer_id)" class="p-2 hover:bg-nord-4 dark:hover:bg-nord-2 rounded text-nord-3 dark:text-nord-4 transition-colors" title="Copy Peer ID">
              <Copy class="w-4 h-4" />
            </button>
          </div>
        </div>

        <!-- Stats -->
        <div class="grid grid-cols-2 gap-4">
          <div class="p-3 bg-nord-5 dark:bg-nord-0 rounded-md border border-nord-4 dark:border-nord-1">
            <div class="text-xs text-nord-3 dark:text-nord-4 mb-1">Connected Peers</div>
            <div class="text-2xl font-bold text-nord-0 dark:text-nord-6">{{ info.peers }}</div>
          </div>
          <div class="p-3 bg-nord-5 dark:bg-nord-0 rounded-md border border-nord-4 dark:border-nord-1">
             <div class="text-xs text-nord-3 dark:text-nord-4 mb-1">Gateway</div>
             <div class="text-sm font-bold text-nord-0 dark:text-nord-6 truncate">http://127.0.0.1:8080</div>
          </div>
        </div>

        <!-- Addresses -->
        <div v-if="info.addresses && info.addresses.length > 0">
           <label class="block text-xs uppercase text-nord-3 dark:text-nord-4 font-bold mb-1">Swarm Addresses</label>
           <div class="bg-nord-5 dark:bg-nord-0 rounded border border-nord-4 dark:border-nord-1 max-h-32 overflow-y-auto">
              <div v-for="addr in info.addresses" :key="addr" class="px-2 py-1 text-xs font-mono text-nord-2 dark:text-nord-5 border-b border-nord-4 dark:border-nord-1 last:border-0">
                {{ addr }}
              </div>
           </div>
        </div>
      </div>

      <div class="mt-6 flex justify-end">
        <button @click="close" class="px-4 py-2 bg-nord-3 dark:bg-nord-1 hover:bg-nord-2 dark:hover:bg-nord-0 text-white rounded-md transition-colors text-sm font-medium">
          Close
        </button>
      </div>
    </div>
  </div>
</template>
