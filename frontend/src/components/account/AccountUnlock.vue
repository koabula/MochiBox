<script setup lang="ts">
import { ref } from 'vue';
import { useAccountStore } from '@/stores/account';
import { useToastStore } from '@/stores/toast';

const accountStore = useAccountStore();
const toastStore = useToastStore();

const password = ref('');
const rememberMe = ref(false);
const loading = ref(false);

const handleUnlock = async () => {
    if (!password.value) return;
    
    loading.value = true;
    try {
        await accountStore.unlock(password.value, rememberMe.value);
    } catch (e: any) {
        toastStore.error(e.response?.data?.error || 'Failed to unlock');
        password.value = '';
    } finally {
        loading.value = false;
    }
};
</script>

<template>
  <div class="fixed inset-0 z-[100] bg-nord-6 dark:bg-nord-0 flex items-center justify-center p-4">
      <div class="bg-white dark:bg-nord-1 w-full max-w-md p-8 rounded-2xl shadow-2xl border border-nord-4 dark:border-nord-3 animate-fade-in">
          <div class="text-center mb-8">
              <div class="w-16 h-16 mx-auto mb-4 bg-nord-10 rounded-2xl flex items-center justify-center shadow-lg shadow-nord-10/30">
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8 text-white" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect><path d="M7 11V7a5 5 0 0 1 10 0v4"></path></svg>
              </div>
              <h1 class="text-2xl font-bold text-nord-0 dark:text-nord-6">Welcome Back</h1>
              <p class="text-nord-3 dark:text-nord-4 mt-2" v-if="accountStore.profile">
                  <span class="font-medium text-nord-1 dark:text-nord-5">{{ accountStore.profile.name }}</span>
              </p>
              <p class="text-nord-3 dark:text-nord-4 mt-1 text-sm">Please unlock your MochiBox</p>
          </div>
          
          <form @submit.prevent="handleUnlock" class="space-y-6">
              <div>
                  <input 
                    v-model="password"
                    type="password" 
                    placeholder="Enter Password"
                    class="w-full px-4 py-3 rounded-xl border border-nord-4 dark:border-nord-3 bg-nord-6 dark:bg-nord-0 text-nord-0 dark:text-nord-6 focus:ring-2 focus:ring-nord-10 outline-none transition-all placeholder-nord-3 dark:placeholder-nord-4"
                    autofocus
                  />
              </div>
              
              <div class="flex items-center gap-2">
                  <input type="checkbox" id="remember" v-model="rememberMe" class="w-4 h-4 rounded border-nord-4 dark:border-nord-3 text-nord-10 focus:ring-nord-10 bg-white dark:bg-nord-0" />
                  <label for="remember" class="text-sm text-nord-3 dark:text-nord-4 select-none cursor-pointer">Remember me on this device</label>
              </div>
              
              <button 
                type="submit" 
                :disabled="loading"
                class="w-full py-3 bg-nord-10 hover:bg-nord-9 text-white font-bold rounded-xl transition-colors disabled:opacity-50 shadow-lg shadow-nord-10/20"
              >
                <span v-if="loading" class="flex items-center justify-center gap-2">
                    <span class="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></span>
                    Unlocking...
                </span>
                <span v-else>Unlock</span>
              </button>
          </form>
      </div>
  </div>
</template>
