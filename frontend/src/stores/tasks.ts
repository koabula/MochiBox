import { defineStore } from 'pinia';
import { ref } from 'vue';

export interface Task {
  id: string;
  type: 'upload' | 'download';
  name: string;
  progress: number; // 0-100
  speed: number; // bytes per second
  status: 'pending' | 'running' | 'completed' | 'error';
  error?: string;
  loaded: number;
  total: number;
  startTime: number;
}

export const useTaskStore = defineStore('tasks', () => {
  const tasks = ref<Task[]>([]);

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

  function updateProgress(id: string, loaded: number, total: number) {
    const task = tasks.value.find(t => t.id === id);
    if (!task) return;

    if (task.status === 'pending') {
      task.status = 'running';
      task.startTime = Date.now();
    }

    const now = Date.now();
    const duration = (now - task.startTime) / 1000; // seconds
    
    // Calculate speed (bytes/sec) - Simple moving average or instantaneous
    // Ideally we track previous loaded and time, but for simplicity:
    // If we just use total / duration, it's average speed.
    // Let's use instantaneous if possible, but store update is discrete.
    // For now, Average speed is safer to avoid spikes.
    if (duration > 0) {
      task.speed = loaded / duration;
    }

    task.loaded = loaded;
    task.total = total;
    if (total > 0) {
      task.progress = Math.min(100, Math.round((loaded / total) * 100));
    }
  }

  function completeTask(id: string) {
    const task = tasks.value.find(t => t.id === id);
    if (task) {
      task.status = 'completed';
      task.progress = 100;
      task.speed = 0;
    }
  }

  function failTask(id: string, error: string) {
    const task = tasks.value.find(t => t.id === id);
    if (task) {
      task.status = 'error';
      task.error = error;
      task.speed = 0;
    }
  }

  function clearCompleted() {
    tasks.value = tasks.value.filter(t => t.status !== 'completed');
  }

  function removeTask(id: string) {
      tasks.value = tasks.value.filter(t => t.id !== id);
  }

  return { tasks, addTask, updateProgress, completeTask, failTask, clearCompleted, removeTask };
});
