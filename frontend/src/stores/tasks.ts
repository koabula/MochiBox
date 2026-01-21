import { defineStore } from 'pinia';
import { ref } from 'vue';
import api from '@/api';
import { taskWebSocket } from '@/api/websocket';

export interface Task {
  id: string;
  type: 'upload' | 'download';
  name: string;
  progress: number;
  speed: number;
  status: 'pending' | 'running' | 'paused' | 'completed' | 'error' | 'canceled';
  error?: string;
  loaded: number;
  total: number;
  startTime: number;
}

export const useTaskStore = defineStore('tasks', () => {
  const tasks = ref<Task[]>([]);

  taskWebSocket.connect();

  function addTask(type: 'upload' | 'download', name: string): string {
    const id = Date.now().toString() + Math.random().toString().slice(2, 6);
    tasks.value.unshift({
      id,
      type,
      name,
      progress: 0,
      speed: 0,
      status: 'pending',
      loaded: 0,
      total: 0,
      startTime: Date.now(),
    });
    return id;
  }

  function addTaskWithID(id: string, type: 'upload' | 'download', name: string) {
    const existing = tasks.value.find(t => t.id === id);
    if (existing) return;
    
    tasks.value.unshift({
      id,
      type,
      name,
      progress: 0,
      speed: 0,
      status: 'pending',
      loaded: 0,
      total: 0,
      startTime: Date.now(),
    });
  }

  function getTask(id: string) {
    return tasks.value.find(t => t.id === id);
  }

  function updateTaskFromBackend(id: string, dto: any) {
    const task = getTask(id);
    if (!task) return;

    const status = String(dto?.status || '');
    if (['running', 'paused', 'completed', 'error', 'canceled'].includes(status)) {
      task.status = status as any;
    }

    if (dto?.error) {
      task.error = String(dto.error);
    }

    task.loaded = Number(dto?.loaded || 0);
    task.total = Number(dto?.total || 0);
    task.speed = Number(dto?.speed || 0);
    
    if (task.total > 0) {
      task.progress = Math.min(100, Math.round((task.loaded / task.total) * 100));
    }
  }

  async function startDownload(fileId: number, filename: string, password?: string) {
    try {
      const res = await api.post('/tasks/download/start', {
        file_id: fileId,
        password: password || '',
      });
      
      const dto = res.data as any;
      const id = String(dto?.id || '');
      
      if (!id) {
        throw new Error('Backend did not return task id');
      }

      addTaskWithID(id, 'download', filename);
      updateTaskFromBackend(id, dto);

      taskWebSocket.subscribe(id, (data: any) => {
        updateTaskFromBackend(id, data);
      });

      return id;
    } catch (e: any) {
      throw new Error(e?.response?.data?.error || e?.message || 'Download failed');
    }
  }

  async function pauseTask(id: string) {
    try {
      const res = await api.post(`/tasks/download/${id}/pause`);
      updateTaskFromBackend(id, res.data);
    } catch (e: any) {
      console.error('Pause failed:', e);
    }
  }

  async function resumeTask(id: string) {
    try {
      const res = await api.post(`/tasks/download/${id}/resume`);
      updateTaskFromBackend(id, res.data);
      
      taskWebSocket.subscribe(id, (data: any) => {
        updateTaskFromBackend(id, data);
      });
    } catch (e: any) {
      console.error('Resume failed:', e);
    }
  }

  async function cancelTask(id: string) {
    try {
      taskWebSocket.unsubscribe(id);
      await api.post(`/tasks/download/${id}/cancel`);
      tasks.value = tasks.value.filter(t => t.id !== id);
    } catch (e: any) {
      console.error('Cancel failed:', e);
    }
  }

  function clearCompleted() {
    const completedIds = tasks.value
      .filter(t => t.status === 'completed' || t.status === 'canceled')
      .map(t => t.id);
    
    completedIds.forEach(id => taskWebSocket.unsubscribe(id));
    tasks.value = tasks.value.filter(t => t.status !== 'completed' && t.status !== 'canceled');
  }

  function removeTask(id: string) {
    taskWebSocket.unsubscribe(id);
    api.post(`/tasks/download/${id}/cancel`).catch(() => {});
    tasks.value = tasks.value.filter(t => t.id !== id);
  }

  function updateProgress(id: string, loaded: number, total: number) {
    const task = getTask(id);
    if (!task) return;
    task.loaded = loaded;
    task.total = total;
    if (total > 0) {
      task.progress = Math.min(100, Math.round((loaded / total) * 100));
    }
    task.status = 'running';
  }

  function completeTask(id: string) {
    const task = getTask(id);
    if (!task) return;
    task.status = 'completed';
    task.progress = 100;
  }

  function failTask(id: string, message: string) {
    const task = getTask(id);
    if (!task) return;
    task.status = 'error';
    task.error = message;
  }

  return {
    tasks,
    addTask,
    getTask,
    startDownload,
    pauseTask,
    resumeTask,
    cancelTask,
    clearCompleted,
    removeTask,
    updateProgress,
    completeTask,
    failTask,
  };
});
