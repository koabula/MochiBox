import { defineStore } from 'pinia';
import { ref } from 'vue';

export interface Toast {
  id: number;
  message: string;
  type: 'success' | 'error' | 'info';
}

export const useToastStore = defineStore('toast', () => {
  const toasts = ref<Toast[]>([]);
  let nextId = 1;

  function show(message: string, type: 'success' | 'error' | 'info' = 'info') {
    const id = nextId++;
    toasts.value.push({ id, message, type });
    setTimeout(() => {
      remove(id);
    }, 3000);
  }

  function success(message: string) {
    show(message, 'success');
  }

  function error(message: string) {
    show(message, 'error');
  }

  function remove(id: number) {
    const index = toasts.value.findIndex(t => t.id === id);
    if (index > -1) {
      toasts.value.splice(index, 1);
    }
  }

  return { toasts, show, success, error, remove };
});
