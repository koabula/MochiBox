<script setup lang="ts">
import { useTaskStore, type Task } from '@/stores/tasks';
import { storeToRefs } from 'pinia';
import { X, CheckCircle, AlertCircle, Upload, Download, Trash2, Activity } from 'lucide-vue-next';
import { computed } from 'vue';

const taskStore = useTaskStore();
const { tasks } = storeToRefs(taskStore);

const formatSpeed = (bytesPerSec: number) => {
    if (bytesPerSec === 0) return '0 B/s';
    const k = 1024;
    const sizes = ['B/s', 'KB/s', 'MB/s', 'GB/s'];
    const i = Math.floor(Math.log(bytesPerSec) / Math.log(k));
    return parseFloat((bytesPerSec / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
};

const formatSize = (bytes: number) => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
};

const pendingTasks = computed(() => tasks.value.filter(t => t.status === 'pending' || t.status === 'running'));
const completedTasks = computed(() => tasks.value.filter(t => t.status === 'completed' || t.status === 'error'));

</script>

<template>
  <div class="h-full flex flex-col p-8 space-y-6 animate-fade-in">
    
    <!-- Header -->
    <div class="flex items-center justify-between">
        <div class="space-y-1">
            <h2 class="text-2xl font-bold text-nord-1 dark:text-nord-6 flex items-center gap-3">
                <Activity class="w-8 h-8 text-nord-10" />
                Task Manager
            </h2>
            <p class="text-nord-3 dark:text-nord-4">Monitor file transfers and background operations.</p>
        </div>
        <button 
            v-if="completedTasks.length > 0"
            @click="taskStore.clearCompleted"
            class="text-sm text-nord-3 hover:text-nord-10 dark:text-nord-4 dark:hover:text-nord-8 flex items-center gap-1"
        >
            <Trash2 class="w-4 h-4" /> Clear Completed
        </button>
    </div>

    <!-- Active Tasks -->
    <div class="space-y-4">
        <h3 class="font-bold text-lg text-nord-1 dark:text-nord-5 uppercase text-sm tracking-wider">Active Tasks ({{ pendingTasks.length }})</h3>
        
        <div v-if="pendingTasks.length === 0" class="p-8 text-center border-2 border-dashed border-nord-4 dark:border-nord-2 rounded-xl text-nord-3 dark:text-nord-4">
            No active tasks currently running.
        </div>

        <div v-for="task in pendingTasks" :key="task.id" class="bg-white dark:bg-nord-1 p-4 rounded-xl shadow-sm border border-nord-4 dark:border-nord-2 space-y-3">
            <div class="flex justify-between items-start">
                <div class="flex items-center gap-3">
                    <div class="p-2 rounded-lg bg-nord-6 dark:bg-nord-2">
                        <Upload v-if="task.type === 'upload'" class="w-5 h-5 text-nord-10" />
                        <Download v-else class="w-5 h-5 text-nord-10" />
                    </div>
                    <div>
                        <p class="font-bold text-nord-1 dark:text-nord-6 truncate max-w-xs">{{ task.name }}</p>
                        <p class="text-xs text-nord-3 dark:text-nord-4 flex items-center gap-2">
                            <span v-if="task.status === 'pending'">Waiting...</span>
                            <span v-else>{{ formatSize(task.loaded) }} / {{ formatSize(task.total) }}</span>
                            <span v-if="task.status === 'running'" class="text-nord-10 font-mono">
                                {{ formatSpeed(task.speed) }}
                            </span>
                        </p>
                    </div>
                </div>
                <button @click="taskStore.removeTask(task.id)" class="text-nord-3 hover:text-red-500">
                    <X class="w-4 h-4" />
                </button>
            </div>
            
            <!-- Progress Bar -->
            <div class="h-2 w-full bg-nord-6 dark:bg-nord-3 rounded-full overflow-hidden">
                <div 
                    class="h-full bg-nord-10 transition-all duration-300 ease-out"
                    :style="{ width: task.progress + '%' }"
                ></div>
            </div>
        </div>
    </div>

    <!-- Completed Tasks -->
    <div v-if="completedTasks.length > 0" class="space-y-4 pt-4">
        <h3 class="font-bold text-lg text-nord-1 dark:text-nord-5 uppercase text-sm tracking-wider">History</h3>
        
        <div class="space-y-2">
            <div v-for="task in completedTasks" :key="task.id" class="flex items-center justify-between p-3 bg-nord-6 dark:bg-nord-1 rounded-lg border border-transparent dark:border-nord-2 hover:border-nord-4 dark:hover:border-nord-3 transition-colors">
                <div class="flex items-center gap-3 min-w-0">
                    <div v-if="task.status === 'completed'" class="text-green-500">
                        <CheckCircle class="w-5 h-5" />
                    </div>
                    <div v-else class="text-red-500">
                        <AlertCircle class="w-5 h-5" />
                    </div>
                    
                    <div class="min-w-0">
                        <p class="font-medium text-sm text-nord-1 dark:text-nord-6 truncate">{{ task.name }}</p>
                        <p v-if="task.error" class="text-xs text-red-500 truncate">{{ task.error }}</p>
                        <p v-else class="text-xs text-nord-3 dark:text-nord-4">{{ task.type === 'upload' ? 'Upload' : 'Download' }} Complete</p>
                    </div>
                </div>
                <button @click="taskStore.removeTask(task.id)" class="text-nord-3 hover:text-nord-1 dark:hover:text-white p-1">
                    <X class="w-4 h-4" />
                </button>
            </div>
        </div>
    </div>

  </div>
</template>
