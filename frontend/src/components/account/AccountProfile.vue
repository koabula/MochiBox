<script setup lang="ts">
import { computed, ref } from 'vue';
import { useAccountStore } from '@/stores/account';
import { LogOut, Shield, Key, Copy, Eye, EyeOff } from 'lucide-vue-next';
import { useToastStore } from '@/stores/toast';

const accountStore = useAccountStore();
const toastStore = useToastStore();

const profile = computed(() => accountStore.profile);

const showExportModal = ref(false);
const exportPassword = ref('');
const mnemonic = ref('');
const showMnemonic = ref(false);

const showChangePwModal = ref(false);
const oldPassword = ref('');
const newPassword = ref('');
const confirmPassword = ref('');

const copyId = async () => {
    if (!profile.value?.public_key) return;
    try {
        await navigator.clipboard.writeText(profile.value.public_key);
        toastStore.success('ID copied to clipboard');
    } catch (e) {
        toastStore.error('Failed to copy ID');
    }
};

const handleLock = () => {
    accountStore.lock();
};

const handleReset = async () => {
    if (confirm('Are you sure you want to delete your account? This action cannot be undone. All your encrypted files will become permanently inaccessible if you lose your recovery phrase.')) {
        try {
            await accountStore.reset();
            toastStore.success('Account reset successfully');
        } catch (e) {
            toastStore.error('Failed to reset account');
        }
    }
};

const handleExport = async () => {
    if (!exportPassword.value) return;
    try {
        const result = await accountStore.exportMnemonic(exportPassword.value);
        mnemonic.value = result;
        exportPassword.value = '';
        toastStore.success('Mnemonic retrieved successfully');
    } catch (e: any) {
        toastStore.error(e.response?.data?.error || 'Incorrect password');
    }
};

const handleChangePassword = async () => {
    if (!oldPassword.value || !newPassword.value) return;
    if (newPassword.value !== confirmPassword.value) {
        toastStore.error("New passwords do not match");
        return;
    }
    
    try {
        await accountStore.changePassword(oldPassword.value, newPassword.value);
        toastStore.success('Password updated successfully');
        showChangePwModal.value = false;
        oldPassword.value = '';
        newPassword.value = '';
        confirmPassword.value = '';
    } catch (e: any) {
        toastStore.error(e.response?.data?.error || 'Failed to update password');
    }
};

const closeExportModal = () => {
    showExportModal.value = false;
    exportPassword.value = '';
    mnemonic.value = '';
    showMnemonic.value = false;
};

const closeChangePwModal = () => {
    showChangePwModal.value = false;
    oldPassword.value = '';
    newPassword.value = '';
    confirmPassword.value = '';
};

const copyMnemonic = () => {
    navigator.clipboard.writeText(mnemonic.value);
    toastStore.success('Copied to clipboard');
};

</script>

<template>
    <div class="p-8 max-w-2xl mx-auto space-y-8 animate-fade-in">
        
        <!-- Profile Header -->
        <div class="flex items-center gap-6 p-6 bg-nord-6 dark:bg-nord-1 rounded-2xl border border-nord-4 dark:border-nord-2">
            <img :src="profile?.avatar" class="w-24 h-24 rounded-full bg-white dark:bg-nord-3 border-2 border-nord-4 dark:border-nord-3 shadow-sm" />
            
            <div class="flex-1 min-w-0">
                <h2 class="text-2xl font-bold text-nord-1 dark:text-nord-6 truncate">{{ profile?.name }}</h2>
                <div class="flex items-center gap-2 mt-2 group cursor-pointer" @click="copyId">
                    <p class="text-sm font-mono text-nord-3 dark:text-nord-4 truncate max-w-[300px] bg-white dark:bg-nord-0 px-2 py-1 rounded border border-nord-4 dark:border-nord-3">
                        {{ profile?.public_key }}
                    </p>
                    <Copy class="w-4 h-4 text-nord-3 group-hover:text-nord-10 transition-colors" />
                </div>
            </div>
            
            <button @click="handleLock" class="px-4 py-2 border border-nord-11 text-nord-11 rounded-lg hover:bg-nord-11 hover:text-white transition-colors flex items-center gap-2">
                <LogOut class="w-4 h-4" /> Lock
            </button>
        </div>

        <!-- Security Settings -->
        <div class="space-y-4">
             <h3 class="text-lg font-bold text-nord-1 dark:text-nord-6 flex items-center gap-2">
                 <Shield class="w-5 h-5 text-nord-10" /> Security
             </h3>
             
             <div class="grid grid-cols-1 gap-4">
                 <div class="p-4 bg-white dark:bg-nord-1 border border-nord-4 dark:border-nord-2 rounded-xl flex items-center justify-between">
                     <div>
                         <h4 class="font-bold text-nord-1 dark:text-nord-5">Export Private Key</h4>
                         <p class="text-xs text-nord-3 dark:text-nord-4 mt-1">View your mnemonic phrase or private key.</p>
                     </div>
                     <button @click="showExportModal = true" class="px-4 py-2 bg-nord-6 dark:bg-nord-2 text-nord-3 dark:text-nord-4 rounded-lg text-sm font-medium hover:text-nord-10 transition-colors">
                         View
                     </button>
                 </div>
                 
                 <div class="p-4 bg-white dark:bg-nord-1 border border-nord-4 dark:border-nord-2 rounded-xl flex items-center justify-between">
                     <div>
                         <h4 class="font-bold text-nord-1 dark:text-nord-5">Change Password</h4>
                         <p class="text-xs text-nord-3 dark:text-nord-4 mt-1">Update your account unlock password.</p>
                     </div>
                     <button @click="showChangePwModal = true" class="px-4 py-2 bg-nord-6 dark:bg-nord-2 text-nord-3 dark:text-nord-4 rounded-lg text-sm font-medium hover:text-nord-10 transition-colors">
                         Update
                     </button>
                 </div>

                 <!-- Danger Zone -->
                 <div class="p-4 bg-red-50 dark:bg-red-900/10 border border-red-200 dark:border-red-900/30 rounded-xl flex items-center justify-between mt-4">
                     <div>
                         <h4 class="font-bold text-red-600 dark:text-red-400">Reset Account</h4>
                         <p class="text-xs text-red-400 dark:text-red-300 mt-1">Delete local account data. Requires re-importing mnemonic.</p>
                     </div>
                     <button @click="handleReset" class="px-4 py-2 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-lg text-sm font-medium hover:bg-red-600 hover:text-white transition-colors">
                         Reset
                     </button>
                 </div>
             </div>
        </div>

        <!-- Export Modal -->
        <div v-if="showExportModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
            <div class="bg-white dark:bg-nord-1 rounded-2xl shadow-xl w-full max-w-md overflow-hidden">
                <div class="px-6 py-4 border-b border-nord-4 dark:border-nord-2 flex justify-between items-center bg-nord-6 dark:bg-nord-2">
                    <h3 class="font-bold text-lg text-nord-1 dark:text-nord-6">Export Private Key</h3>
                    <button @click="closeExportModal" class="text-nord-3 hover:text-nord-1 dark:text-nord-4 dark:hover:text-white">
                        <span class="sr-only">Close</span>
                        <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
                    </button>
                </div>
                
                <div class="p-6 space-y-4">
                    <div v-if="!mnemonic">
                        <p class="text-sm text-nord-3 dark:text-nord-4 mb-4">Please enter your password to reveal your recovery phrase.</p>
                        <input 
                            v-model="exportPassword" 
                            type="password" 
                            placeholder="Current Password" 
                            class="w-full px-3 py-2 rounded-lg border border-nord-4 dark:border-nord-3 bg-white dark:bg-nord-0 text-sm focus:ring-2 focus:ring-nord-10 outline-none"
                            @keyup.enter="handleExport"
                        />
                        <button 
                            @click="handleExport"
                            :disabled="!exportPassword"
                            class="w-full mt-4 py-2.5 bg-nord-10 hover:bg-nord-9 disabled:opacity-50 disabled:cursor-not-allowed text-white font-bold rounded-lg transition-colors"
                        >
                            Reveal
                        </button>
                    </div>
                    
                    <div v-else class="space-y-4 animate-fade-in">
                        <div class="bg-nord-6 dark:bg-nord-0 p-4 rounded-xl border border-nord-4 dark:border-nord-2 relative group">
                            <p class="font-mono text-center text-lg font-bold text-nord-1 dark:text-nord-6 break-words" :class="{'blur-sm select-none': !showMnemonic}">
                                {{ mnemonic }}
                            </p>
                            <button @click="showMnemonic = !showMnemonic" class="absolute top-2 right-2 p-1 text-nord-3 hover:text-nord-10">
                                <Eye v-if="!showMnemonic" class="w-4 h-4" />
                                <EyeOff v-else class="w-4 h-4" />
                            </button>
                        </div>
                        <div class="flex gap-2">
                            <button @click="copyMnemonic" class="flex-1 py-2.5 bg-nord-4 dark:bg-nord-3 hover:bg-nord-5 dark:hover:bg-nord-2 text-nord-1 dark:text-nord-6 font-bold rounded-lg transition-colors flex items-center justify-center gap-2">
                                <Copy class="w-4 h-4" /> Copy
                            </button>
                            <button @click="closeExportModal" class="flex-1 py-2.5 bg-nord-10 hover:bg-nord-9 text-white font-bold rounded-lg transition-colors">
                                Done
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Change Password Modal -->
        <div v-if="showChangePwModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
            <div class="bg-white dark:bg-nord-1 rounded-2xl shadow-xl w-full max-w-md overflow-hidden">
                <div class="px-6 py-4 border-b border-nord-4 dark:border-nord-2 flex justify-between items-center bg-nord-6 dark:bg-nord-2">
                    <h3 class="font-bold text-lg text-nord-1 dark:text-nord-6">Change Password</h3>
                    <button @click="closeChangePwModal" class="text-nord-3 hover:text-nord-1 dark:text-nord-4 dark:hover:text-white">
                        <span class="sr-only">Close</span>
                        <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
                    </button>
                </div>
                
                <div class="p-6 space-y-4">
                    <div class="space-y-3">
                        <div>
                            <label class="text-xs font-bold text-nord-3 dark:text-nord-4 uppercase">Current Password</label>
                            <input v-model="oldPassword" type="password" class="w-full mt-1 px-3 py-2 rounded-lg border border-nord-4 dark:border-nord-3 bg-white dark:bg-nord-0 text-sm focus:ring-2 focus:ring-nord-10 outline-none" />
                        </div>
                        <div>
                            <label class="text-xs font-bold text-nord-3 dark:text-nord-4 uppercase">New Password</label>
                            <input v-model="newPassword" type="password" class="w-full mt-1 px-3 py-2 rounded-lg border border-nord-4 dark:border-nord-3 bg-white dark:bg-nord-0 text-sm focus:ring-2 focus:ring-nord-10 outline-none" />
                        </div>
                        <div>
                            <label class="text-xs font-bold text-nord-3 dark:text-nord-4 uppercase">Confirm New Password</label>
                            <input v-model="confirmPassword" type="password" class="w-full mt-1 px-3 py-2 rounded-lg border border-nord-4 dark:border-nord-3 bg-white dark:bg-nord-0 text-sm focus:ring-2 focus:ring-nord-10 outline-none" />
                        </div>
                    </div>

                    <button 
                        @click="handleChangePassword"
                        :disabled="!oldPassword || !newPassword || !confirmPassword"
                        class="w-full mt-2 py-2.5 bg-nord-10 hover:bg-nord-9 disabled:opacity-50 disabled:cursor-not-allowed text-white font-bold rounded-lg transition-colors"
                    >
                        Update Password
                    </button>
                </div>
            </div>
        </div>

    </div>
</template>
