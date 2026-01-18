import { defineStore } from 'pinia';
import api from '@/api';

interface Account {
    id: number;
    public_key: string;
    name: string;
    avatar: string;
}

interface AccountState {
    configured: boolean;
    locked: boolean;
    profile: Account | null;
    loading: boolean;
}

export const useAccountStore = defineStore('account', {
    state: (): AccountState => ({
        configured: false,
        locked: true,
        profile: null,
        loading: true,
    }),

    actions: {
        async checkStatus() {
            this.loading = true;
            try {
                const res = await api.get('/account/status');
                this.configured = res.data.configured;
                this.locked = res.data.locked;
                this.profile = res.data.profile;
            } catch (e) {
                console.error("Failed to check account status", e);
            } finally {
                this.loading = false;
            }
        },

        async generateMnemonic() {
            const res = await api.post('/account/generate-mnemonic');
            return res.data.mnemonic;
        },

        async initAccount(mnemonic: string, password: string, name: string) {
            await api.post('/account/init', { mnemonic, password, name });
            await this.checkStatus();
        },

        async unlock(password: string, rememberMe: boolean) {
            await api.post('/account/unlock', { password, remember_me: rememberMe });
            await this.checkStatus();
        },

        async lock() {
            await api.post('/account/lock');
            await this.checkStatus();
        },

        async reset() {
            await api.delete('/account/');
            await this.checkStatus();
        },

        async exportMnemonic(password: string) {
            const res = await api.post('/account/export', { password });
            return res.data.mnemonic;
        },

        async changePassword(oldPassword: string, newPassword: string) {
            await api.post('/account/change-password', { old_password: oldPassword, new_password: newPassword });
        }
    }
});
