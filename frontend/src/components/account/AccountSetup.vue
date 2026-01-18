<script setup lang="ts">
import { ref, computed } from 'vue';
import { useAccountStore } from '@/stores/account';
import { useToastStore } from '@/stores/toast';
import { Plus, Download, AlertTriangle } from 'lucide-vue-next';

const accountStore = useAccountStore();
const toastStore = useToastStore();

const step = ref(1);
const mode = ref<'create' | 'import'>('create');
const mnemonic = ref('');
const mnemonicInput = ref('');
const submitting = ref(false);

const form = ref({
    name: '',
    password: '',
    confirmPassword: ''
});

const mnemonicWords = computed(() => mnemonic.value ? mnemonic.value.split(' ') : []);

const startCreate = async () => {
    try {
        mnemonic.value = await accountStore.generateMnemonic();
        mode.value = 'create';
        step.value = 2;
    } catch (e) {
        toastStore.error('Failed to generate mnemonic');
    }
};

const startImport = () => {
    mode.value = 'import';
    step.value = 2;
};

const validateImport = () => {
    const words = mnemonicInput.value.trim().split(/\s+/);
    if (words.length !== 12 && words.length !== 24) {
        toastStore.error(`Invalid mnemonic length: ${words.length} words (expected 12 or 24)`);
        return;
    }
    mnemonic.value = mnemonicInput.value.trim();
    step.value = 3;
};

const submit = async () => {
    if (!form.value.name || !form.value.password) {
        toastStore.error('Please fill all fields');
        return;
    }
    if (form.value.password !== form.value.confirmPassword) {
        return;
    }

    submitting.value = true;
    try {
        await accountStore.initAccount(mnemonic.value, form.value.password, form.value.name);
        toastStore.success('Account created successfully!');
    } catch (e: any) {
        toastStore.error(e.response?.data?.error || 'Failed to create account');
    } finally {
        submitting.value = false;
    }
};
const downloadMnemonic = () => {
    if (!mnemonic.value) return;
    const blob = new Blob([mnemonic.value], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'mochibox-recovery.txt';
    a.click();
    URL.revokeObjectURL(url);
    toastStore.success('Recovery phrase downloaded. Store it securely offline!');
};
</script>

<template>
    <div class="fixed inset-0 z-[100] bg-nord-6 dark:bg-nord-0 flex items-center justify-center p-4">
        <div class="bg-white dark:bg-nord-1 w-full max-w-2xl p-8 rounded-2xl shadow-2xl border border-nord-4 dark:border-nord-3 flex flex-col h-[600px] animate-fade-in">
            <!-- Header -->
            <div class="text-center mb-8 flex-shrink-0">
                 <h1 class="text-2xl font-bold text-nord-0 dark:text-nord-6">Setup MochiBox Account</h1>
                 <p class="text-nord-3 dark:text-nord-4 mt-2">Create a secure identity for decentralized sharing</p>
            </div>

            <!-- Steps -->
            <div class="flex-1 overflow-y-auto px-1">
                <!-- Step 1: Choice -->
                <div v-if="step === 1" class="grid grid-cols-2 gap-6 h-full items-center content-center pb-8">
                    <button @click="startCreate" class="h-64 p-6 border-2 border-nord-4 dark:border-nord-3 rounded-2xl hover:border-nord-10 hover:bg-nord-6 dark:hover:bg-nord-2 transition-all flex flex-col items-center justify-center gap-4 group">
                        <div class="w-16 h-16 bg-nord-10/10 text-nord-10 rounded-full flex items-center justify-center group-hover:scale-110 transition-transform">
                            <Plus class="w-8 h-8" />
                        </div>
                        <h3 class="text-lg font-bold text-nord-1 dark:text-nord-5">Create New Account</h3>
                        <p class="text-sm text-nord-3 text-center">Generate a new secure wallet and identity.</p>
                    </button>
                    
                    <button @click="startImport" class="h-64 p-6 border-2 border-nord-4 dark:border-nord-3 rounded-2xl hover:border-nord-10 hover:bg-nord-6 dark:hover:bg-nord-2 transition-all flex flex-col items-center justify-center gap-4 group">
                         <div class="w-16 h-16 bg-orange-500/10 text-orange-500 rounded-full flex items-center justify-center group-hover:scale-110 transition-transform">
                            <Download class="w-8 h-8" />
                        </div>
                        <h3 class="text-lg font-bold text-nord-1 dark:text-nord-5">Import Account</h3>
                        <p class="text-sm text-nord-3 text-center">Restore from existing 12/24 word mnemonic.</p>
                    </button>
                </div>

                <!-- Step 2: Mnemonic (Create) -->
                <div v-if="step === 2 && mode === 'create'" class="space-y-6">
                    <div class="bg-amber-50 dark:bg-amber-900/20 p-4 rounded-xl border border-amber-200 dark:border-amber-800 flex gap-3">
                        <AlertTriangle class="w-6 h-6 text-amber-500 flex-shrink-0" />
                        <div class="text-sm text-amber-800 dark:text-amber-200">
                            <p class="font-bold">Save these words securely!</p>
                            <p>This mnemonic phrase is the ONLY way to recover your account. If you lose it, your files are lost forever.</p>
                        </div>
                    </div>
                    
                    <div class="grid grid-cols-4 gap-3">
                        <div v-for="(word, i) in mnemonicWords" :key="i" class="bg-nord-6 dark:bg-nord-0 p-2 rounded border border-nord-4 dark:border-nord-3 flex items-center gap-2">
                            <span class="text-xs text-nord-3 font-mono select-none w-5">{{ i+1 }}</span>
                            <span class="font-mono font-bold text-nord-1 dark:text-nord-5 select-all">{{ word }}</span>
                        </div>
                    </div>
                    
                    <div class="flex justify-center pt-8 gap-4">
                         <button @click="step = 1" class="px-6 py-3 text-nord-3 hover:text-nord-1">Back</button>
                        <button @click="downloadMnemonic" class="px-6 py-3 border border-nord-4 dark:border-nord-3 text-nord-1 dark:text-nord-5 font-bold rounded-xl hover:bg-nord-6 dark:hover:bg-nord-2 flex items-center gap-2">
                            <Download class="w-4 h-4" /> Download Backup
                        </button>
                        <button @click="step = 3" class="px-8 py-3 bg-nord-10 text-white font-bold rounded-xl hover:bg-nord-9">
                            I have saved them
                        </button>
                    </div>
                </div>

                <!-- Step 2: Mnemonic (Import) -->
                 <div v-if="step === 2 && mode === 'import'" class="space-y-6">
                    <div>
                        <label class="block text-sm font-bold text-nord-3 mb-2 dark:text-nord-4">Enter Mnemonic Phrase</label>
                        <textarea 
                            v-model="mnemonicInput"
                            placeholder="apple banana ..."
                            class="w-full h-32 p-4 rounded-xl border border-nord-4 dark:border-nord-3 bg-nord-6 dark:bg-nord-0 font-mono resize-none focus:ring-2 focus:ring-nord-10 outline-none text-nord-0 dark:text-nord-6"
                        ></textarea>
                        <p class="text-xs text-nord-3 mt-2">Separate words with spaces. Supports 12 or 24 words.</p>
                    </div>
                     <div class="flex justify-end pt-4 gap-4">
                        <button @click="step = 1" class="px-6 py-2 text-nord-3 hover:text-nord-1 dark:text-nord-4 dark:hover:text-white">Back</button>
                        <button @click="validateImport" class="px-8 py-3 bg-nord-10 text-white font-bold rounded-xl hover:bg-nord-9">
                            Next
                        </button>
                    </div>
                 </div>

                 <!-- Step 3: Profile -->
                 <div v-if="step === 3" class="space-y-6 max-w-sm mx-auto pt-4">
                     <div>
                         <label class="block text-sm font-bold text-nord-3 mb-2 dark:text-nord-4">Account Name</label>
                         <input v-model="form.name" type="text" placeholder="Your Name" class="w-full px-4 py-3 rounded-xl border border-nord-4 dark:border-nord-3 bg-nord-6 dark:bg-nord-0 focus:ring-2 focus:ring-nord-10 outline-none text-nord-0 dark:text-nord-6" />
                     </div>
                     <div>
                         <label class="block text-sm font-bold text-nord-3 mb-2 dark:text-nord-4">Set Password</label>
                         <input v-model="form.password" type="password" placeholder="Password" class="w-full px-4 py-3 rounded-xl border border-nord-4 dark:border-nord-3 bg-nord-6 dark:bg-nord-0 focus:ring-2 focus:ring-nord-10 outline-none text-nord-0 dark:text-nord-6" />
                     </div>
                     <div>
                         <label class="block text-sm font-bold text-nord-3 mb-2 dark:text-nord-4">Confirm Password</label>
                         <input v-model="form.confirmPassword" type="password" placeholder="Confirm Password" class="w-full px-4 py-3 rounded-xl border border-nord-4 dark:border-nord-3 bg-nord-6 dark:bg-nord-0 focus:ring-2 focus:ring-nord-10 outline-none text-nord-0 dark:text-nord-6" />
                         <p v-if="form.confirmPassword && form.password !== form.confirmPassword" class="text-xs text-red-500 mt-1">Passwords do not match</p>
                     </div>
                     
                     <div class="pt-6 flex gap-4">
                         <button @click="step = 2" class="px-6 py-3 text-nord-3 hover:text-nord-1 dark:text-nord-4 dark:hover:text-white">Back</button>
                         <button @click="submit" :disabled="submitting" class="flex-1 py-3 bg-nord-10 text-white font-bold rounded-xl hover:bg-nord-9 disabled:opacity-50">
                             <span v-if="submitting">Creating Account...</span>
                             <span v-else>Finish Setup</span>
                         </button>
                     </div>
                 </div>
            </div>
        </div>
    </div>
</template>
