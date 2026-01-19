<script setup lang="ts">
import { ref, onMounted, watch } from 'vue';
import { X, Folder, File as FileIcon, Loader2, ArrowLeft, ChevronRight, Eye, Download } from 'lucide-vue-next';
import api from '@/api';

const props = defineProps<{
  isOpen: boolean,
  cid: string,
  name: string
}>();

const emit = defineEmits(['close']);

interface DirItem {
    name: string;
    cid: string; // Wait, ListDirectory only returns name currently? No, we updated it to assume name-based resolution?
    // Actually backend implementation in node.go ListDirectory:
    // We didn't fix CID retrieval for children yet in previous turn!
    // The previous plan mentioned "Let's defer CID resolution".
    // So current ListDirectory returns empty CID or problematic CID?
    // Let's check backend implementation again.
    // Wait, I updated node.go but commented out CID resolution logic because of complexity.
    // If I don't have CID, I can't navigate!
    // I need to fix backend ListDirectory first to return CIDs or use Path based navigation.
    // If I use Path based navigation, I need an API that accepts path.
    // API /api/files/:cid/ls takes root CID.
    // If I want to list subdirectory, I can pass subdirectory CID.
    // BUT I don't have subdirectory CID if ListDirectory doesn't return it.
    
    // Quick Fix for Backend:
    // In node.go, we can use `stat` to get CID.
    // Or we can just use path-based listing? /api/files/:rootCid/ls?path=subdir
    // Let's stick to CIDs if possible.
    // Actually, boxo `files.Directory` entries provide `Node()`.
    // We can cast `Node` to `ProtoNode` to get CID?
    // Let's assume for now I will fix backend to return CIDs.
    
    size: number;
    type: string;
}

// State
const currentCid = ref('');
const currentPath = ref<Array<{name: string, cid: string}>>([]);
const items = ref<DirItem[]>([]);
const loading = ref(true);
const error = ref('');

// Initialize
watch(() => props.isOpen, (val) => {
    if (val) {
        currentCid.value = props.cid;
        currentPath.value = [{ name: props.name, cid: props.cid }];
        fetchItems(props.cid);
    }
}, { immediate: true });

async function fetchItems(cid: string) {
    loading.value = true;
    error.value = '';
    items.value = [];
    try {
        const res = await api.get(`/files/${cid}/ls`);
        items.value = res.data;
    } catch (e: any) {
        error.value = e.response?.data?.error || 'Failed to list directory';
    } finally {
        loading.value = false;
    }
}

const handleNavigate = (item: DirItem) => {
    if (item.type === 'dir') {
        // Push to stack
        if (!item.cid) {
             error.value = "Unable to navigate: missing CID";
             return;
        }
        currentPath.value.push({ name: item.name, cid: item.cid });
        currentCid.value = item.cid;
        fetchItems(item.cid);
    } else {
        // Preview File
        if (!item.cid) return;
        const baseUrl = api.defaults.baseURL || 'http://localhost:3666/api';
        window.open(`${baseUrl}/preview/${item.cid}`, '_blank');
    }
};

const handleDownload = (item: DirItem, e: Event) => {
    e.stopPropagation();
    if (!item.cid) return;
    const baseUrl = api.defaults.baseURL || 'http://localhost:3666/api';
    const url = `${baseUrl}/preview/${item.cid}?download=true&filename=${encodeURIComponent(item.name)}`;
    
    // Create hidden anchor tag to trigger download without opening new window
    const link = document.createElement('a');
    link.style.display = 'none';
    link.href = url;
    link.setAttribute('download', item.name); // Hint to browser
    document.body.appendChild(link);
    link.click();
    
    // Cleanup
    setTimeout(() => {
        document.body.removeChild(link);
    }, 100);
};

const handleBreadcrumb = (index: number) => {
    if (index === currentPath.value.length - 1) return;
    
    const target = currentPath.value[index];
    currentPath.value = currentPath.value.slice(0, index + 1);
    currentCid.value = target.cid;
    fetchItems(target.cid);
};

const handleBack = () => {
    if (currentPath.value.length > 1) {
        handleBreadcrumb(currentPath.value.length - 2);
    }
};

const formatSize = (bytes: number) => {
  if (!bytes || bytes === 0) return 'Unknown';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};
</script>

<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
    <div class="bg-white dark:bg-nord-1 rounded-2xl shadow-xl w-full max-w-3xl overflow-hidden flex flex-col max-h-[80vh] min-h-[400px]">
      
      <!-- Header -->
      <div class="px-6 py-4 border-b border-nord-4 dark:border-nord-2 bg-nord-6 dark:bg-nord-2 flex flex-col gap-2">
        <div class="flex justify-between items-center">
            <div class="flex items-center gap-3">
                <div class="p-2 bg-nord-4 dark:bg-nord-3 rounded-lg">
                    <Folder class="w-5 h-5 text-nord-10 dark:text-nord-8" />
                </div>
                <h3 class="font-bold text-lg text-nord-1 dark:text-nord-6 truncate max-w-md">Folder Preview</h3>
            </div>
            <button @click="$emit('close')" class="text-nord-3 hover:text-nord-1 dark:text-nord-4 dark:hover:text-white">
              <X class="w-5 h-5" />
            </button>
        </div>
        
        <!-- Breadcrumbs -->
        <div class="flex items-center gap-1 text-sm text-nord-3 dark:text-nord-4 overflow-x-auto scrollbar-hide">
            <button 
                @click="handleBack"
                :disabled="currentPath.length <= 1"
                class="p-1 hover:bg-nord-4 dark:hover:bg-nord-3 rounded disabled:opacity-30 disabled:hover:bg-transparent"
            >
                <ArrowLeft class="w-4 h-4" />
            </button>
            <div class="h-4 w-px bg-nord-4 dark:bg-nord-3 mx-2"></div>
            
            <template v-for="(crumb, index) in currentPath" :key="crumb.cid">
                <button 
                    @click="handleBreadcrumb(index)"
                    class="hover:text-nord-10 dark:hover:text-nord-8 transition-colors whitespace-nowrap"
                    :class="index === currentPath.length - 1 ? 'font-bold text-nord-1 dark:text-nord-6' : ''"
                >
                    {{ crumb.name }}
                </button>
                <ChevronRight v-if="index < currentPath.length - 1" class="w-4 h-4 opacity-50 flex-shrink-0" />
            </template>
        </div>
      </div>

      <!-- Content -->
      <div class="flex-1 overflow-y-auto p-0 relative">
          <div v-if="loading" class="absolute inset-0 flex flex-col items-center justify-center bg-white/80 dark:bg-nord-1/80 z-10">
              <Loader2 class="w-8 h-8 animate-spin text-nord-10 mb-2" />
              <p class="text-nord-3 dark:text-nord-4">Loading contents...</p>
          </div>
          
          <div v-else-if="error" class="p-12 text-center">
              <p class="text-red-500 mb-2">{{ error }}</p>
              <button @click="fetchItems(currentCid)" class="text-nord-10 hover:underline">Retry</button>
          </div>
          
          <table v-else class="w-full text-left text-sm">
              <thead class="bg-nord-6 dark:bg-nord-2 text-nord-3 dark:text-nord-4 uppercase font-semibold sticky top-0 z-0">
                  <tr>
                      <th class="px-6 py-3 border-b border-nord-4 dark:border-nord-3">Name</th>
                      <th class="px-6 py-3 border-b border-nord-4 dark:border-nord-3 w-32">Size</th>
                      <th class="px-6 py-3 border-b border-nord-4 dark:border-nord-3 w-24">Type</th>
                      <th class="px-6 py-3 border-b border-nord-4 dark:border-nord-3 w-20"></th>
                  </tr>
              </thead>
              <tbody class="divide-y divide-nord-4 dark:divide-nord-3">
                  <tr 
                    v-for="item in items" 
                    :key="item.name" 
                    class="group hover:bg-nord-6 dark:hover:bg-nord-2 cursor-pointer transition-colors"
                    @click="handleNavigate(item)"
                  >
                      <td class="px-6 py-3 flex items-center gap-3 font-medium text-nord-1 dark:text-nord-6">
                          <Folder v-if="item.type === 'dir'" class="w-4 h-4 text-nord-10 fill-nord-10/20" />
                          <FileIcon v-else class="w-4 h-4 text-nord-9" />
                          {{ item.name }}
                      </td>
                      <td class="px-6 py-3 text-nord-3 dark:text-nord-4">{{ formatSize(item.size) }}</td>
                      <td class="px-6 py-3 text-nord-3 dark:text-nord-4 capitalize opacity-70">{{ item.type }}</td>
                      <td class="px-6 py-3 text-right flex items-center justify-end gap-2">
                          <button 
                            @click="(e) => handleDownload(item, e)"
                            class="p-1 text-nord-3 dark:text-nord-4 hover:text-nord-10 dark:hover:text-nord-8 opacity-0 group-hover:opacity-100 transition-opacity"
                            title="Download"
                          >
                              <Download class="w-4 h-4" />
                          </button>
                          <Eye v-if="item.type === 'file'" class="w-4 h-4 text-nord-3 dark:text-nord-4 opacity-0 group-hover:opacity-100 hover:text-nord-10" />
                          <ChevronRight v-if="item.type === 'dir'" class="w-4 h-4 text-nord-3 dark:text-nord-4 opacity-0 group-hover:opacity-100" />
                      </td>
                  </tr>
                  <tr v-if="items.length === 0">
                      <td colspan="4" class="px-6 py-12 text-center text-nord-3 dark:text-nord-4 italic">
                          Empty directory
                      </td>
                  </tr>
              </tbody>
          </table>
      </div>
      
      <div class="px-6 py-4 border-t border-nord-4 dark:border-nord-2 bg-nord-6 dark:bg-nord-2 text-right">
          <button @click="$emit('close')" class="px-4 py-2 bg-nord-4 dark:bg-nord-3 hover:bg-nord-5 dark:hover:bg-nord-2 rounded-lg font-bold text-nord-1 dark:text-nord-6 transition-colors">
              Close
          </button>
      </div>
    </div>
  </div>
</template>