<script setup lang="ts">
import { ref } from 'vue';
import { FileText, Download, Trash2, Eye, Share2, Pin, RefreshCcw, Lock, Globe, UserCheck, Copy, Check } from 'lucide-vue-next';
import PasswordInputModal from './PasswordInputModal.vue';
import { useToastStore } from '@/stores/toast';

const props = defineProps<{
  files: any[],
  showClearHistory?: boolean,
  showPin?: boolean,
  showSync?: boolean
}>();

const emit = defineEmits(['preview', 'delete', 'share', 'download', 'clear-history', 'pin', 'sync']);
const toast = useToastStore();

const showPasswordModal = ref(false);
const selectedFile = ref<any>(null);
const actionType = ref<'preview' | 'download'>('preview');
const copiedKey = ref<string | null>(null);

const handleActionClick = (file: any, type: 'preview' | 'download') => {
    if (file.encryption_type === 'password' && !file.saved_password) {
        selectedFile.value = file;
        actionType.value = type;
        showPasswordModal.value = true;
    } else {
        emit(type, file);
    }
};

const handlePasswordSubmit = (password: string) => {
    showPasswordModal.value = false;
    if (selectedFile.value) {
        // Emit a copy with password attached
        emit(actionType.value, { ...selectedFile.value, password });
        selectedFile.value = null;
    }
};

const truncateKey = (key: string) => {
    if (!key || key.length < 12) return key;
    return `${key.slice(0, 6)}...${key.slice(-6)}`;
};

const copyKey = async (key: string) => {
    try {
        await navigator.clipboard.writeText(key);
        copiedKey.value = key;
        setTimeout(() => copiedKey.value = null, 2000);
        toast.success('Public Key copied to clipboard');
    } catch (e) {
        toast.error('Failed to copy key');
    }
};

const formatSize = (bytes: number) => {
  if (!bytes || bytes === 0) return 'Unknown';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

const formatDate = (dateStr: string) => {
  return new Date(dateStr).toLocaleDateString();
};
</script>

<template>
  <div class="overflow-x-auto bg-white dark:bg-nord-1 rounded-xl shadow-sm border border-nord-4 dark:border-nord-2">
    <table class="w-full text-left text-sm">
      <thead class="bg-nord-6 dark:bg-nord-2 text-nord-3 dark:text-nord-4 uppercase tracking-wider font-semibold border-b border-nord-4 dark:border-nord-3">
        <tr>
          <th class="px-6 py-4">Name</th>
          <th class="px-6 py-4">Size</th>
          <th class="px-6 py-4">Security</th>
          <th class="px-6 py-4">Type</th>
          <th class="px-6 py-4">Date</th>
          <th class="px-6 py-4 text-right whitespace-nowrap flex items-center justify-end gap-2">
            Actions
            <button 
                v-if="showSync"
                @click="$emit('sync')"
                class="p-1.5 rounded-lg text-nord-3 hover:text-nord-10 hover:bg-nord-5 dark:hover:bg-nord-3 transition-colors"
                title="Sync from Node"
            >
                <RefreshCcw class="w-4 h-4" />
            </button>
            <button 
                v-if="showClearHistory && files.length > 0"
                @click="$emit('clear-history')"
                class="p-1.5 rounded-lg text-nord-3 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/10 transition-colors"
                title="Clear All History"
            >
                <Trash2 class="w-4 h-4" />
            </button>
          </th>
        </tr>
      </thead>
      <tbody class="divide-y divide-nord-4 dark:divide-nord-3">
        <tr v-for="file in files" :key="file.id" class="hover:bg-nord-6 dark:hover:bg-nord-2 transition-colors">
          <td class="px-6 py-4 font-medium text-nord-1 dark:text-nord-6 flex items-center gap-3">
            <div class="p-2 bg-nord-5 dark:bg-nord-3 rounded-lg">
              <FileText class="w-5 h-5 text-nord-10 dark:text-nord-8" />
            </div>
            {{ file.name }}
          </td>
          <td class="px-6 py-4 text-nord-3 dark:text-nord-4">{{ formatSize(file.size) }}</td>
          <td class="px-6 py-4">
              <div class="flex items-center gap-2">
                  <div v-if="file.encryption_type === 'password'" class="flex items-center gap-1.5 px-2 py-1 bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-500 rounded text-xs font-bold uppercase">
                      <Lock class="w-3 h-3" /> Password
                  </div>
                  <div 
                      v-else-if="file.encryption_type === 'private'" 
                      @click.stop="copyKey(file.recipient_pub_key)"
                      class="group relative flex items-center gap-1.5 px-2 py-1 bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-500 rounded text-xs font-bold uppercase cursor-pointer hover:bg-purple-200 dark:hover:bg-purple-900/50 transition-colors"
                  >
                      <UserCheck class="w-3 h-3" /> Private
                      <!-- Custom Tooltip -->
                      <div class="hidden group-hover:block absolute bottom-full left-1/2 -translate-x-1/2 mb-1.5 px-3 py-2 bg-nord-4 dark:bg-nord-0 text-nord-0 dark:text-white text-xs rounded-lg shadow-xl z-50 whitespace-nowrap border border-nord-3 dark:border-nord-2">
                          <div class="flex items-center gap-2">
                              <span class="font-mono opacity-90">{{ truncateKey(file.recipient_pub_key) }}</span>
                              <div class="p-1 rounded transition-colors opacity-70">
                                  <Check v-if="copiedKey === file.recipient_pub_key" class="w-3 h-3 text-green-600 dark:text-green-400" />
                                  <Copy v-else class="w-3 h-3" />
                              </div>
                          </div>
                          <!-- Arrow -->
                          <div class="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-nord-4 dark:border-t-nord-0"></div>
                      </div>
                  </div>
                  <div v-else class="flex items-center gap-1.5 px-2 py-1 bg-nord-4 dark:bg-nord-3 text-nord-2 dark:text-nord-4 rounded text-xs font-bold uppercase">
                      <Globe class="w-3 h-3" /> Public
                  </div>
              </div>
          </td>
          <td class="px-6 py-4 text-nord-3 dark:text-nord-4 truncate max-w-[150px]" :title="file.mime_type">{{ file.mime_type }}</td>
          <td class="px-6 py-4 text-nord-3 dark:text-nord-4">{{ formatDate(file.created_at) }}</td>
          <td class="px-6 py-4 text-right space-x-2 whitespace-nowrap">
             <button @click="handleActionClick(file, 'preview')" class="p-2 text-nord-3 hover:text-nord-10 dark:text-nord-4 dark:hover:text-nord-8 transition-colors" title="Preview">
              <Eye class="w-4 h-4" />
            </button>
             <button @click="handleActionClick(file, 'download')" class="p-2 text-nord-3 hover:text-nord-10 dark:text-nord-4 dark:hover:text-nord-8 transition-colors" title="Download">
              <Download class="w-4 h-4" />
            </button>
             <button @click="$emit('share', file)" class="p-2 text-nord-3 hover:text-nord-10 dark:text-nord-4 dark:hover:text-nord-8 transition-colors" title="Share">
              <Share2 class="w-4 h-4" />
            </button>
             <button v-if="showPin" @click="$emit('pin', file)" class="p-2 text-nord-3 hover:text-nord-10 dark:text-nord-4 dark:hover:text-nord-8 transition-colors" title="Pin to Local Node">
              <Pin class="w-4 h-4" />
            </button>
            <button @click="$emit('delete', file.id)" class="p-2 text-nord-3 hover:text-red-500 dark:text-nord-4 dark:hover:text-red-400 transition-colors" title="Delete">
              <Trash2 class="w-4 h-4" />
            </button>
          </td>
        </tr>
        <tr v-if="files.length === 0">
            <td colspan="5" class="px-6 py-12 text-center text-nord-3 dark:text-nord-4 opacity-70">
                No files found. Upload something!
            </td>
        </tr>
      </tbody>
    </table>
    <PasswordInputModal 
      :is-open="showPasswordModal" 
      :file-name="selectedFile?.name || ''"
      @close="showPasswordModal = false"
      @submit="handlePasswordSubmit"
    />
  </div>
</template>