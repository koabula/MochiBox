import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import api from '@/api';
import { useSettingsStore } from './settings';

export const useNetworkStore = defineStore('network', () => {
    const status = ref({
        online: false,
        peers: 0,
        peer_id: '',
        addresses: [] as string[],
        data_dir: ''
    });
    
    const pollInterval = ref<any>(null);

    async function fetchStatus() {
        try {
            const res = await api.get('/system/status');
            // If backend returns 200 but online:false, it means node is stopped or starting.
            status.value = res.data;
        } catch (e) {
            // Connection error to backend
            status.value = { online: false, peers: 0, peer_id: '', addresses: [], data_dir: '' };
        }
    }

    function startPolling(interval = 3000) {
        if (pollInterval.value) return;
        fetchStatus();
        pollInterval.value = setInterval(fetchStatus, interval);
    }

    function stopPolling() {
        if (pollInterval.value) {
            clearInterval(pollInterval.value);
            pollInterval.value = null;
        }
    }

    // Computed: Is the node supposedly starting?
    // True if: Embedded is enabled in settings, but we are offline.
    const isStarting = computed(() => {
        const settingsStore = useSettingsStore();
        return settingsStore.useEmbeddedNode && !status.value.online;
    });

    return { status, fetchStatus, startPolling, stopPolling, isStarting };
});
