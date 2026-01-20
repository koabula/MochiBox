import { defineStore } from 'pinia';
import { ref } from 'vue';
import api from '@/api';
import { useTaskStore } from './tasks';

export interface FileItem {
  id: number;
  cid: string;
  name: string;
  size: number;
  mime_type: string;
  encryption_type: string;
  saved_password?: string;
  recipient_pub_key?: string;
  is_folder: boolean;
  created_at: string;
}

export const useFileStore = defineStore('files', () => {
  const files = ref<FileItem[]>([]);
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

  async function uploadFile(input: File | File[], options: { encryptionType: string, password?: string, receiverPubKey?: string, savePassword?: boolean, useLocalFile?: boolean } = { encryptionType: 'public' }) {
    const taskStore = useTaskStore();
    
    let name = '';
    const formData = new FormData();

    if (Array.isArray(input)) {
        if (input.length === 0) return;
        // Folder
        // Try to get folder name from first file path
        const firstPath = input[0].webkitRelativePath;
        const parts = firstPath.split('/');
        name = parts.length > 1 ? parts[0] : 'Folder';
        
        for (const f of input) {
            formData.append('file', f);
            formData.append('paths[]', f.webkitRelativePath);
        }
    } else {
        name = input.name;
        // Check for local file optimization (No Copy)
        // @ts-ignore
        if (options.useLocalFile && input.path) {
             // @ts-ignore
             formData.append('file_path', input.path);
             formData.append('use_local', 'true');
             // Do NOT append file content
        } else {
             formData.append('file', input);
        }
    }

    const taskId = taskStore.addTask('upload', name);
    
    formData.append('encryption_type', options.encryptionType);
    if (options.password) formData.append('password', options.password);
    if (options.receiverPubKey) formData.append('receiver_pub_key', options.receiverPubKey);
    if (options.savePassword) formData.append('save_password', 'true');

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
