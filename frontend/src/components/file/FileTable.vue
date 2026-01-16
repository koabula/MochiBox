<script setup lang="ts">
import { FileText, Download, Trash2, Eye, Share2, Pin } from 'lucide-vue-next';

const props = defineProps<{
  files: any[],
  showClearHistory?: boolean,
  showPin?: boolean
}>();

const emit = defineEmits(['preview', 'delete', 'share', 'download', 'clear-history', 'pin']);

const formatSize = (bytes: number) => {
  if (bytes === 0) return '0 B';
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
          <th class="px-6 py-4">Type</th>
          <th class="px-6 py-4">Date</th>
          <th class="px-6 py-4 text-right whitespace-nowrap flex items-center justify-end gap-2">
            Actions
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
          <td class="px-6 py-4 text-nord-3 dark:text-nord-4">{{ file.mime_type }}</td>
          <td class="px-6 py-4 text-nord-3 dark:text-nord-4">{{ formatDate(file.created_at) }}</td>
          <td class="px-6 py-4 text-right space-x-2 whitespace-nowrap">
             <button @click="$emit('preview', file)" class="p-2 text-nord-3 hover:text-nord-10 dark:text-nord-4 dark:hover:text-nord-8 transition-colors" title="Preview">
              <Eye class="w-4 h-4" />
            </button>
             <button @click="$emit('download', file)" class="p-2 text-nord-3 hover:text-nord-10 dark:text-nord-4 dark:hover:text-nord-8 transition-colors" title="Download">
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
  </div>
</template>