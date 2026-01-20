import { defineStore } from 'pinia';
import { ref } from 'vue';

export interface Toast {
  id: number;
  message: string;
  type: 'success' | 'error' | 'info' | 'warning';
}

export const useToastStore = defineStore('toast', () => {
  const toasts = ref<Toast[]>([]);
  let nextId = 1;

  function show(message: string, type: 'success' | 'error' | 'info' | 'warning' = 'info'): number {
    const id = nextId++;
    toasts.value.push({ id, message, type });
    setTimeout(() => {
      remove(id);
    }, 3000);
    return id;
  }

  function success(message: string): number {
    return show(message, 'success');
  }

  function error(message: string): number {
    return show(message, 'error');
  }

  function info(message: string): number {
    return show(message, 'info');
  }

  function warning(message: string): number {
    return show(message, 'warning');
  }

  function remove(id: number) {
    const index = toasts.value.findIndex(t => t.id === id);
    if (index > -1) {
      toasts.value.splice(index, 1);
    }
  }

  return { toasts, show, success, error, info, warning, remove };
});
