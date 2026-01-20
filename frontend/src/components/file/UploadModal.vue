<script setup lang="ts">
import { ref } from 'vue';
import { X, Upload, File as FileIcon, Lock, Globe, UserCheck, Folder } from 'lucide-vue-next';

const props = defineProps<{
  isOpen: boolean
}>();

const emit = defineEmits(['close', 'upload']);

const fileInput = ref<HTMLInputElement | null>(null);
const folderInput = ref<HTMLInputElement | null>(null);
const selectedFile = ref<File | null>(null);
const selectedFiles = ref<File[]>([]);
const encryptionType = ref('public');
const password = ref('');
const receiverPubKey = ref('');
const savePassword = ref(false);

import { watch } from 'vue';
watch(() => props.isOpen, (newVal) => {
    if (newVal) reset();
});

const handleFileSelect = (event: Event) => {
  const target = event.target as HTMLInputElement;
  if (target.files && target.files.length > 0) {
    selectedFile.value = target.files[0];
    selectedFiles.value = [];
  }
};

const handleFolderSelect = (event: Event) => {
  const target = event.target as HTMLInputElement;
  if (target.files && target.files.length > 0) {
    selectedFiles.value = Array.from(target.files);
    
    // Create preview
    const first = target.files[0];
    const path = first.webkitRelativePath;
    const name = path.split('/')[0] || 'Folder';
    const totalSize = selectedFiles.value.reduce((acc, f) => acc + f.size, 0);
    
    // Mock file for display
    selectedFile.value = {
        name: name,
        size: totalSize
    } as File;
  }
};

const handleDrop = (event: DragEvent) => {
  event.preventDefault();
  if (event.dataTransfer?.items) {
      // Check for folder drop? Browsers make this hard without FileSystem API.
      // Standard drop gives files. If a folder is dropped, Chrome gives the files inside?
      // No, usually it gives the folder as a File with size 0 or type "".
      // Handling drag-and-drop folders is complex (webkitGetAsEntry).
      // For now, let's assume standard file drop or use the button for folders.
      // If user drops multiple files, we can handle that?
      // MochiBox current MVP handles single file drop.
      // Let's keep it simple for now.
  }
  
  if (event.dataTransfer?.files && event.dataTransfer.files.length > 0) {
    selectedFile.value = event.dataTransfer.files[0];
    selectedFiles.value = [];
  }
};

const handleSubmit = () => {
  if (!selectedFile.value) return;
  
  const payload = selectedFiles.value.length > 0 ? selectedFiles.value : selectedFile.value;
  
  emit('upload', { 
      file: payload,
      options: {
          encryptionType: encryptionType.value,
          password: password.value,
          receiverPubKey: receiverPubKey.value,
          savePassword: savePassword.value
      }
  });
};

const reset = () => {
  selectedFile.value = null;
  selectedFiles.value = [];
  encryptionType.value = 'public';
  password.value = '';
  receiverPubKey.value = '';
  savePassword.value = false;
};

const close = () => {
    reset();
    emit('close');
}
</script>

<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm transition-opacity">
    <div class="bg-white dark:bg-nord-1 rounded-2xl shadow-xl w-full max-w-md overflow-hidden transform transition-all scale-100">
      
      <div class="px-6 py-4 border-b border-nord-4 dark:border-nord-2 flex justify-between items-center bg-nord-6 dark:bg-nord-2">
        <h3 class="font-bold text-lg text-nord-1 dark:text-nord-6">Upload File</h3>
        <button @click="close" class="text-nord-3 hover:text-nord-1 dark:text-nord-4 dark:hover:text-white">
          <X class="w-5 h-5" />
        </button>
      </div>

      <div class="p-6 space-y-6">
        
        <!-- Drop Zone -->
        <div 
          v-if="!selectedFile"
          @dragover.prevent
          @drop="handleDrop"
          class="border-2 border-dashed border-nord-4 dark:border-nord-3 rounded-xl p-8 flex flex-col items-center justify-center gap-3 hover:border-nord-10 dark:hover:border-nord-8 hover:bg-nord-6 dark:hover:bg-nord-2 transition-all"
        >
          <div class="p-3 bg-nord-5 dark:bg-nord-3 rounded-full">
            <Upload class="w-6 h-6 text-nord-10 dark:text-nord-8" />
          </div>
          <div class="text-center">
            <p class="font-medium text-nord-1 dark:text-nord-6">Drag file here or select</p>
            <p class="text-xs text-nord-3 dark:text-nord-4 mt-1">Any file type supported</p>
          </div>
          
          <div class="flex gap-3 mt-2">
            <button @click.stop="fileInput?.click()" class="px-4 py-2 bg-nord-4 dark:bg-nord-3 hover:bg-nord-5 dark:hover:bg-nord-2 rounded-lg text-sm font-bold text-nord-1 dark:text-nord-6 transition-colors flex items-center gap-2">
                <FileIcon class="w-4 h-4" />
                Select File
            </button>
            <button @click.stop="folderInput?.click()" class="px-4 py-2 bg-nord-4 dark:bg-nord-3 hover:bg-nord-5 dark:hover:bg-nord-2 rounded-lg text-sm font-bold text-nord-1 dark:text-nord-6 transition-colors flex items-center gap-2">
                <Folder class="w-4 h-4" />
                Select Folder
            </button>
          </div>
          
          <input ref="fileInput" type="file" class="hidden" @change="handleFileSelect" />
          <input ref="folderInput" type="file" webkitdirectory class="hidden" @change="handleFolderSelect" />
        </div>

        <!-- Selected File Preview -->
        <div v-else class="space-y-4">
            <div class="flex items-center gap-4 p-4 bg-nord-6 dark:bg-nord-2 rounded-lg border border-nord-4 dark:border-nord-3">
              <div class="p-2 bg-nord-4 dark:bg-nord-3 rounded">
                <FileIcon class="w-6 h-6 text-nord-10" />
              </div>
              <div class="flex-1 min-w-0">
                <p class="font-medium text-sm truncate text-nord-1 dark:text-nord-6">{{ selectedFile.name }}</p>
                <p class="text-xs text-nord-3 dark:text-nord-4">{{ (selectedFile.size / 1024).toFixed(1) }} KB</p>
              </div>
              <button @click="selectedFile = null" class="text-nord-3 hover:text-red-500">
                <X class="w-4 h-4" />
              </button>
            </div>

            <!-- Encryption Options -->
            <div class="p-4 bg-nord-5 dark:bg-nord-2 rounded-xl border border-nord-4 dark:border-nord-3 space-y-4">
                <label class="text-xs font-bold text-nord-3 dark:text-nord-4 uppercase">Encryption Mode</label>
                <div class="flex p-1 bg-nord-4 dark:bg-nord-3 rounded-lg">
                    <button 
                        v-for="type in ['public', 'password', 'private']"
                        :key="type"
                        @click="encryptionType = type"
                        :class="[
                            'flex-1 py-1.5 text-xs font-bold rounded-md transition-all uppercase flex items-center justify-center gap-2',
                            encryptionType === type 
                                ? 'bg-white dark:bg-nord-1 text-nord-10 shadow-sm' 
                                : 'text-nord-3 dark:text-nord-4 hover:text-nord-1 dark:hover:text-nord-6'
                        ]"
                    >
                        <Globe v-if="type === 'public'" class="w-3 h-3" />
                        <Lock v-if="type === 'password'" class="w-3 h-3" />
                        <UserCheck v-if="type === 'private'" class="w-3 h-3" />
                        {{ type }}
                    </button>
                </div>
                
                <div v-if="encryptionType === 'password'" class="space-y-2 animate-fade-in">
                    <input v-model="password" type="password" placeholder="Set Password" class="w-full px-3 py-2 rounded-lg border border-nord-4 dark:border-nord-3 bg-white dark:bg-nord-0 text-sm focus:ring-2 focus:ring-nord-10 outline-none text-nord-0 dark:text-nord-6" />
                    <label class="flex items-center gap-2 cursor-pointer select-none">
                        <input type="checkbox" v-model="savePassword" class="rounded border-nord-4 dark:border-nord-3 text-nord-10 focus:ring-nord-10 bg-white dark:bg-nord-0" />
                        <span class="text-xs text-nord-3 dark:text-nord-4">Save password locally (auto-fill for me)</span>
                    </label>
                    <p class="text-xs text-nord-3 dark:text-nord-4">Password will be required to decrypt this file.</p>
                </div>
                
                <div v-if="encryptionType === 'private'" class="space-y-2 animate-fade-in">
                    <input v-model="receiverPubKey" type="text" placeholder="Receiver Public Key (ID)" class="w-full px-3 py-2 rounded-lg border border-nord-4 dark:border-nord-3 bg-white dark:bg-nord-0 text-sm font-mono focus:ring-2 focus:ring-nord-10 outline-none text-nord-0 dark:text-nord-6" />
                    <p class="text-xs text-nord-3 dark:text-nord-4">Only the specified user can decrypt this file.</p>
                </div>
                <div v-if="encryptionType === 'public' && !selectedFiles.length" class="mt-4 animate-fade-in">
                    <label class="flex items-center gap-2 cursor-pointer select-none p-2 bg-nord-4 dark:bg-nord-3 rounded-lg hover:bg-nord-5 dark:hover:bg-nord-2 transition-colors">
                        <input type="checkbox" v-model="useLocalFile" class="rounded border-nord-4 dark:border-nord-3 text-nord-10 focus:ring-nord-10 bg-white dark:bg-nord-0" />
                        <div>
                            <span class="text-sm font-bold text-nord-1 dark:text-nord-5">Use local file (No Copy)</span>
                            <p class="text-xs text-nord-3 dark:text-nord-4">Recommended for large files. File must remain on disk.</p>
                        </div>
                    </label>
                </div>
            </div>
        </div>

        <!-- Submit Button -->
        <button 
          @click="handleSubmit"
          :disabled="!selectedFile"
          class="w-full py-3 bg-nord-10 hover:bg-nord-9 disabled:opacity-50 disabled:cursor-not-allowed text-white font-bold rounded-xl transition-colors shadow-lg shadow-nord-10/20"
        >
          Upload to IPFS
        </button>

      </div>
    </div>
  </div>
</template>

<style scoped>
.animate-fade-in {
  animation: fadeIn 0.2s ease-out;
}
@keyframes fadeIn {
  from { opacity: 0; transform: translateY(-5px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>
