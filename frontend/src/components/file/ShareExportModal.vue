<script setup lang="ts">
import { Copy, X, Shield, Link } from 'lucide-vue-next';
import { useToastStore } from '@/stores/toast';
import { ref } from 'vue';
import api from '@/api';

const props = defineProps<{
  isOpen: boolean;
  file: any;
}>();

const emit = defineEmits(['close']);
const toast = useToastStore();
const includePassword = ref(false);
const passwordInput = ref('');

import { watch } from 'vue';
watch(() => props.isOpen, (newVal) => {
    if (newVal) {
        includePassword.value = false;
        passwordInput.value = '';
    }
});

watch(includePassword, async (val) => {
    if (val && !passwordInput.value && props.file.saved_password) {
        try {
            const res = await api.post(`/files/${props.file.id}/reveal`);
            passwordInput.value = res.data.password;
        } catch (e) {
            console.error("Failed to fetch password", e);
        }
    }
});

const handleCopy = async () => {
    if (!props.file) return;

    // Construct Mochi Link Payload
    const payload: any = {
        v: 1,
        c: props.file.cid,
        n: props.file.name,
        s: props.file.size,
        t: props.file.encryption_type === 'password' ? 'pwd' : 
           props.file.encryption_type === 'private' ? 'priv' : 'pub'
    };
    if (props.file.mime_type) {
        payload.m = props.file.mime_type;
    }
    
    if (props.file.encryption_meta) {
        payload.p = {};
        if (payload.t === 'pwd') {
            payload.p.salt = props.file.encryption_meta;
            if (includePassword.value && passwordInput.value) {
                // Warning: Sending password in plain text inside the link
                // Ideally this should be obfuscated or just raw
                payload.p.pw = passwordInput.value;
            }
        }
        if (payload.t === 'priv') payload.p.ek = props.file.encryption_meta;
    }
    
    try {
        const jsonStr = JSON.stringify(payload);
        // UTF-8 Safe Base64 Encoding
        const encoder = new TextEncoder();
        const data = encoder.encode(jsonStr);
        // Convert Uint8Array to binary string
        let binary = '';
        const len = data.byteLength;
        for (let i = 0; i < len; i++) {
            binary += String.fromCharCode(data[i]);
        }
        const base64Str = btoa(binary);
        const shareLink = `mochi://${base64Str}`;
        
        await navigator.clipboard.writeText(shareLink);
        toast.success('Mochi Link copied to clipboard!');
        emit('close');
    } catch (e) {
        console.error(e);
        toast.error('Failed to copy');
    }
};

const close = () => {
    emit('close');
};
</script>

<template>
  <div v-if="isOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
    <div class="bg-white dark:bg-nord-1 rounded-2xl shadow-xl w-full max-w-md overflow-hidden">
      
      <div class="px-6 py-4 border-b border-nord-4 dark:border-nord-2 flex justify-between items-center bg-nord-6 dark:bg-nord-2">
        <h3 class="font-bold text-lg text-nord-1 dark:text-nord-6 flex items-center gap-2">
            <Shield class="w-5 h-5 text-nord-10" /> Share File
        </h3>
        <button @click="close" class="text-nord-3 hover:text-nord-1 dark:text-nord-4 dark:hover:text-white">
          <X class="w-5 h-5" />
        </button>
      </div>

      <div class="p-6 space-y-6">
          <div class="p-4 bg-nord-6 dark:bg-nord-2 rounded-xl">
              <h4 class="font-bold text-nord-1 dark:text-nord-6 truncate">{{ file.name }}</h4>
              <p class="text-xs font-mono text-nord-3 dark:text-nord-4 mt-1">{{ file.cid }}</p>
          </div>

          <div v-if="file.encryption_type === 'password'" class="p-4 border border-amber-200 dark:border-amber-900/50 bg-amber-50 dark:bg-amber-900/10 rounded-xl space-y-3">
              <div class="flex items-center gap-2">
                  <input type="checkbox" id="includePw" v-model="includePassword" class="w-4 h-4 text-amber-500 rounded focus:ring-amber-500" />
                  <label for="includePw" class="text-sm font-bold text-amber-800 dark:text-amber-500 select-none">Include Password in Link</label>
              </div>
              <p class="text-xs text-amber-700 dark:text-amber-600">Anyone with this link will be able to decrypt the file without entering a password.</p>
              
              <div v-if="includePassword" class="animate-fade-in">
                  <input v-model="passwordInput" type="password" placeholder="Enter File Password to Include" class="w-full px-3 py-2 rounded-lg border border-amber-300 dark:border-amber-800 bg-white dark:bg-nord-0 text-sm focus:ring-2 focus:ring-amber-500 outline-none" />
              </div>
          </div>

          <button 
            @click="handleCopy"
            class="w-full py-3 bg-nord-10 hover:bg-nord-9 disabled:opacity-50 disabled:cursor-not-allowed text-white font-bold rounded-xl transition-colors shadow-lg shadow-nord-10/20 flex items-center justify-center gap-2"
          >
            <Copy class="w-4 h-4" /> Copy Mochi Link
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
