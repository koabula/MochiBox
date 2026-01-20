import { defineStore } from 'pinia';
import api from '../api';
import { ref } from 'vue';

export const useSharedStore = defineStore('shared', () => {
    const history = ref<any[]>([]);
    const loading = ref(false);
    let pollingInterval: any = null;

    const startPolling = () => {
        if (pollingInterval) return;
        
        // Only poll if we have items with unknown size (0)
        const hasUnknownSize = history.value.some(h => h.size === 0);
        if (!hasUnknownSize) return;

        let attempts = 0;
        pollingInterval = setInterval(async () => {
            attempts++;
            // Stop after 30 attempts (approx 60 seconds) to save resources
            if (attempts > 30) {
                stopPolling();
                return;
            }

            try {
                // Fetch silently (no loading spinner)
                const response = await api.get('/shared/history');
                const newHistory = response.data;
                
                // Check if any size changed
                newHistory.forEach((newItem: any) => {
                    const oldItem = history.value.find(h => h.id === newItem.id);
                    if (oldItem && oldItem.size === 0 && newItem.size > 0) {
                        oldItem.size = newItem.size;
                    }
                });

                // Also detect new items if any? No, just update sizes for now.
                // Or full replace? Full replace is safer but might interrupt user interaction?
                // Updating in place is better for UI stability.
                
                // If all sizes resolved, stop polling
                if (!newHistory.some((h: any) => h.size === 0)) {
                    stopPolling();
                }
            } catch (e) {
                console.warn("Polling failed", e);
            }
        }, 2000);
    };

    const stopPolling = () => {
        if (pollingInterval) {
            clearInterval(pollingInterval);
            pollingInterval = null;
        }
    };

    const fetchHistory = async () => {
        loading.value = true;
        try {
            const response = await api.get('/shared/history');
            history.value = response.data;
            startPolling();
        } catch (error) {
            console.error('Failed to fetch shared history:', error);
        } finally {
            loading.value = false;
        }
    };

    const addToHistory = async (cid: string, name?: string, size?: number, mimeType?: string, encryptionType?: string, encryptionMeta?: string, originalLink?: string) => {
        try {
            const response = await api.post('/shared/history', { 
                cid, 
                name,
                size,
                mime_type: mimeType,
                encryption_type: encryptionType,
                encryption_meta: encryptionMeta,
                original_link: originalLink
            });
            // Add to top of list or replace existing
            const index = history.value.findIndex(h => h.id === response.data.id);
            if (index !== -1) {
                history.value[index] = response.data;
            } else {
                history.value.unshift(response.data);
            }
            
            // Start polling if size is unknown
            if (response.data.size === 0) {
                startPolling();
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
