import { defineStore } from 'pinia';
import api from '../api';
import { ref } from 'vue';

export const useSharedStore = defineStore('shared', () => {
    const history = ref<any[]>([]);
    const loading = ref(false);

    const fetchHistory = async () => {
        loading.value = true;
        try {
            const response = await api.get('/shared/history');
            history.value = response.data;
        } catch (error) {
            console.error('Failed to fetch shared history:', error);
        } finally {
            loading.value = false;
        }
    };

    const addToHistory = async (cid: string, name?: string) => {
        try {
            const response = await api.post('/shared/history', { cid, name });
            // Add to top of list or replace existing
            const index = history.value.findIndex(h => h.id === response.data.id);
            if (index !== -1) {
                history.value[index] = response.data;
            } else {
                history.value.unshift(response.data);
            }
            return response.data;
        } catch (error) {
            console.error('Failed to add to shared history:', error);
            throw error;
        }
    };

    const deleteHistory = async (id: number) => {
        try {
            await api.delete(`/shared/history/${id}`);
            history.value = history.value.filter(h => h.id !== id);
        } catch (error) {
            console.error('Failed to delete history item:', error);
            throw error;
        }
    };

    const clearHistory = async () => {
        try {
            await api.delete('/shared/history');
            history.value = [];
        } catch (error) {
            console.error('Failed to clear history:', error);
            throw error;
        }
    };

    return {
        history,
        loading,
        fetchHistory,
        addToHistory,
        deleteHistory,
        clearHistory
    };
});
