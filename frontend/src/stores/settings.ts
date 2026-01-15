import { defineStore } from 'pinia';
import { ref } from 'vue';
import api from '@/api';

export const useSettingsStore = defineStore('settings', () => {
  const downloadPath = ref('');
  const askPath = ref(false);
  const ipfsApiUrl = ref('http://127.0.0.1:5001');
  const useEmbeddedNode = ref(true);

  async function fetchSettings() {
    try {
      const res = await api.get('/config');
      downloadPath.value = res.data.download_path;
      askPath.value = res.data.ask_path;
      if (res.data.ipfs_api_url) {
        ipfsApiUrl.value = res.data.ipfs_api_url;
      }
      if (res.data.use_embedded_node !== undefined) {
        useEmbeddedNode.value = res.data.use_embedded_node;
      }
    } catch (e) {
      console.error('Failed to fetch settings', e);
    }
  }

  async function updateSettings(path: string, ask: boolean, ipfsUrl?: string, embedded?: boolean) {
    try {
      const payload: any = { 
        download_path: path, 
        ask_path: ask,
        use_embedded_node: embedded !== undefined ? embedded : useEmbeddedNode.value
      };
      if (ipfsUrl) {
          payload.ipfs_api_url = ipfsUrl;
      } else {
          payload.ipfs_api_url = ipfsApiUrl.value;
      }
      
      const res = await api.post('/config', payload);
      downloadPath.value = res.data.download_path;
      askPath.value = res.data.ask_path;
      useEmbeddedNode.value = res.data.use_embedded_node;
      if (res.data.ipfs_api_url) {
        ipfsApiUrl.value = res.data.ipfs_api_url;
      }
    } catch (e) {
      console.error('Failed to update settings', e);
      throw e;
    }
  }

  async function setDataDir(newPath: string) {
      try {
          const res = await api.post('/system/datadir', { new_path: newPath });
          return res.data;
      } catch (e) {
          throw e;
      }
  }

  return { downloadPath, askPath, ipfsApiUrl, useEmbeddedNode, fetchSettings, updateSettings, setDataDir };
});
