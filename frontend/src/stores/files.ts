import { defineStore } from 'pinia';
import { ref } from 'vue';
import api from '@/api';
import { useTaskStore } from './tasks';

export interface File {
  id: number;
  cid: string;
  name: string;
  size: number;
  mime_type: string;
  created_at: string;
}

export const useFileStore = defineStore('files', () => {
  const files = ref<File[]>([]);
  const loading = ref(false);
  const uploading = ref(false);

  async function fetchFiles() {
    loading.value = true;
    try {
      const res = await api.get('/files');
      files.value = res.data;
    } catch (e) {
      console.error(e);
    } finally {
      loading.value = false;
    }
  }

  async function uploadFile(file: File, options: { encryptionType: string, password?: string, receiverPubKey?: string } = { encryptionType: 'public' }) {
    const taskStore = useTaskStore();
    const taskId = taskStore.addTask('upload', file.name);
    
    // uploading.value = true; // No longer block UI
    
    const formData = new FormData();
    formData.append('file', file as any);
    formData.append('encryption_type', options.encryptionType);
    if (options.password) formData.append('password', options.password);
    if (options.receiverPubKey) formData.append('receiver_pub_key', options.receiverPubKey);

    try {
      await api.post('/files/upload', formData, {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
        onUploadProgress: (progressEvent) => {
            if (progressEvent.total) {
                taskStore.updateProgress(taskId, progressEvent.loaded, progressEvent.total);
            }
        }
      });
      taskStore.completeTask(taskId);
      await fetchFiles();
    } catch (e: any) {
      console.error(e);
      taskStore.failTask(taskId, e.message || 'Upload failed');
      throw e;
    } finally {
      // uploading.value = false;
    }
  }

  async function deleteFile(id: number) {
      try {
          await api.delete(`/files/${id}`);
          await fetchFiles();
      } catch (e) {
          console.error(e);
      }
  }

  async function syncFiles() {
    loading.value = true;
    try {
      await api.post('/files/sync');
      await fetchFiles();
    } catch (e) {
      console.error(e);
      throw e;
    } finally {
      loading.value = false;
    }
  }

  return { files, loading, uploading, fetchFiles, uploadFile, deleteFile, syncFiles };
});
