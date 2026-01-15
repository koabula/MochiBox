<script setup lang="ts">
import { Home, Share2, Settings, Box, Sun, Moon, Activity, Globe } from 'lucide-vue-next';
import { ref, onMounted, onUnmounted } from 'vue';
import { useNetworkStore } from '@/stores/network';

const props = defineProps<{
  currentTab: string
}>();

const emit = defineEmits(['update:currentTab']);
const networkStore = useNetworkStore();

const isDark = ref(localStorage.getItem('theme') === 'dark' || document.documentElement.classList.contains('dark'));

onMounted(() => {
    networkStore.startPolling();
});

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

// Init theme on mount
if (isDark.value) {
    document.documentElement.classList.add('dark');
}

const tabs = [
  { id: 'files', label: 'My Files', icon: Home },
  { id: 'shared', label: 'Shared', icon: Share2 },
  { id: 'tasks', label: 'Tasks', icon: Activity },
  { id: 'network', label: 'Network', icon: Globe },
  { id: 'settings', label: 'Settings', icon: Settings },
];
</script>

<template>
  <aside class="w-64 bg-nord-6 dark:bg-nord-1 border-r border-nord-4 dark:border-nord-2 flex flex-col h-full transition-colors duration-300">
    <div class="p-6 flex items-center gap-3">
      <img src="@/assets/icon.png" class="w-8 h-8 rounded-lg shadow-sm" alt="MochiBox" />
      <span class="font-bold text-xl tracking-tight text-nord-0 dark:text-nord-6">MochiBox</span>
    </div>

    <nav class="flex-1 px-4 py-4 space-y-1">
      <button 
        v-for="tab in tabs" 
        :key="tab.id"
        @click="emit('update:currentTab', tab.id)"
        :class="[
          'w-full flex items-center gap-3 px-4 py-3 rounded-lg transition-all duration-200 font-medium',
          currentTab === tab.id 
            ? 'bg-white dark:bg-nord-3 text-nord-10 dark:text-nord-8 shadow-sm' 
            : 'text-nord-3 dark:text-nord-4 hover:bg-nord-5 dark:hover:bg-nord-2'
        ]"
      >
        <component :is="tab.icon" class="w-5 h-5" />
        {{ tab.label }}
      </button>
    </nav>

    <div class="p-4 border-t border-nord-4 dark:border-nord-2 flex flex-col gap-2">
      <!-- Node Status (Quick View) -->
      <div 
        @click="emit('update:currentTab', 'network')"
        class="bg-white dark:bg-nord-3 rounded-lg p-3 text-xs space-y-1 shadow-sm border border-nord-4 dark:border-nord-2 cursor-pointer hover:border-nord-10 dark:hover:border-nord-8 transition-colors group"
      >
          <div class="flex items-center justify-between">
             <span class="font-bold text-nord-1 dark:text-nord-6 group-hover:text-nord-10 transition-colors">IPFS Status</span>
             <span :class="networkStore.status.online ? 'bg-green-500' : (networkStore.isStarting ? 'bg-amber-500 animate-pulse' : 'bg-red-500')" class="w-2 h-2 rounded-full"></span>
          </div>
          <div v-if="networkStore.status.online">
             <div class="text-nord-3 dark:text-nord-4 truncate mt-1 font-mono" :title="networkStore.status.peer_id">
                ID: ...{{ networkStore.status.peer_id.slice(-6) }}
             </div>
             <div class="text-nord-3 dark:text-nord-4">
                Peers: {{ networkStore.status.peers }}
             </div>
          </div>
          <div v-else-if="networkStore.isStarting" class="text-amber-600 dark:text-amber-500 mt-1 font-medium">
             Starting Node...
          </div>
          <div v-else class="text-red-500 mt-1">
             Disconnected
          </div>
      </div>

      <button 
        @click="toggleTheme"
        class="flex items-center gap-3 px-4 py-3 rounded-lg text-nord-3 dark:text-nord-4 hover:bg-nord-5 dark:hover:bg-nord-2 transition-all w-full"
      >
        <component :is="isDark ? Sun : Moon" class="w-5 h-5" />
        <span class="text-sm font-medium">{{ isDark ? 'Light Mode' : 'Dark Mode' }}</span>
      </button>
    </div>
  </aside>
</template>