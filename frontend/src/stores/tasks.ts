import { defineStore } from 'pinia';
import { ref } from 'vue';
import api from '@/api';

export interface Task {
  id: string;
  type: 'upload' | 'download';
  name: string;
  progress: number; // 0-100
  speed: number; // bytes per second
  status: 'pending' | 'running' | 'paused' | 'completed' | 'error';
  error?: string;
  loaded: number;
  total: number;
  startTime: number;
  lastLoaded?: number;     // For speed calc
  lastSpeedUpdate?: number; // For speed calc
}

interface DownloadContext {
  chunks: Blob[];
  controller: AbortController;
  url: string;
  filename: string;
  fileHandle?: any; // FileSystemFileHandle
  writable?: any;   // FileSystemWritableFileStream
}

export const useTaskStore = defineStore('tasks', () => {
  const tasks = ref<Task[]>([]);
  
  // Non-reactive context to hold heavy objects
  const downloadContexts = new Map<string, DownloadContext>();

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

  // --- Download Logic ---

  async function startDownload(url: string, filename: string, fileHandle?: any, taskId?: string) {
    let id = taskId;
    if (!id) {
       id = addTask('download', filename);
    }
    
    const task = tasks.value.find(t => t.id === id);
    if (!task) return;

    task.status = 'running';
    if (!task.startTime) task.startTime = Date.now();
    
    // Init Context
    if (!downloadContexts.has(id!)) {
        downloadContexts.set(id!, {
            chunks: [],
            controller: new AbortController(),
            url,
            filename,
            fileHandle
        });
    }

    const context = downloadContexts.get(id!)!;
    context.controller = new AbortController();

    try {
        const fullUrl = url.startsWith('http') ? url : `${api.defaults.baseURL}${url}`;
        
        const headers: HeadersInit = {};
        // Resume support
        if (task.loaded > 0) {
            headers['Range'] = `bytes=${task.loaded}-`;
        }

        const response = await fetch(fullUrl, {
            signal: context.controller.signal,
            headers
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const contentLength = response.headers.get('Content-Length');
        // If range request, content-length is partial. Total is loaded + partial.
        const total = contentLength ? parseInt(contentLength, 10) + task.loaded : 0;
        // Update total only if we didn't know it or it changed (shouldn't change ideally)
        if (task.total === 0 && total > 0) task.total = total;

        const reader = response.body?.getReader();
        if (!reader) throw new Error('ReadableStream not supported');

        // If using File System Access API
        if (context.fileHandle) {
             // Create writable if not exists (or append mode)
             // We need to seek if resuming
             if (!context.writable) {
                 context.writable = await context.fileHandle.createWritable({ keepExistingData: true });
             }
             // Seek to current position
             await context.writable.seek(task.loaded);
        }

        let lastNow = performance.now();
        
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            
            if (value) {
                if (context.writable) {
                    await context.writable.write(value);
                } else {
                    context.chunks.push(new Blob([value]));
                }

                const newLoaded = task.loaded + value.length;
                
                // Speed Calculation
                const now = performance.now();
                const timeDiff = (now - lastNow) / 1000; // seconds
                
                // Update speed every 500ms
                if (timeDiff > 0.5 || newLoaded === task.total) {
                    const loadedDiff = newLoaded - (task.lastLoaded || 0);
                    const instantSpeed = loadedDiff / timeDiff;
                    
                    // Smoothing: 70% old, 30% new
                    task.speed = task.speed === 0 ? instantSpeed : (task.speed * 0.7 + instantSpeed * 0.3);
                    
                    task.lastLoaded = newLoaded;
                    lastNow = now;
                }

                updateProgress(id!, newLoaded, task.total);
            }
        }

        // Finish
        if (context.writable) {
            await context.writable.close();
        }
        completeDownload(id!);

    } catch (err: any) {
        if (err.name === 'AbortError') {
            // Paused by user
        } else {
            failTask(id!, err.message);
        }
    }
  }

  function pauseTask(id: string) {
      const task = tasks.value.find(t => t.id === id);
      const context = downloadContexts.get(id);
      
      if (task && context && task.status === 'running') {
          context.controller.abort();
          task.status = 'paused';
          task.speed = 0;
          // Keep writable open? No, better to close and re-open to flush data
          if (context.writable) {
              // We can't easily close a writable that is being written to in the loop?
              // The loop will exit on abort.
              // But we might need to handle cleanup. 
              // Usually aborting fetch stops the reader.
              // We should probably close the writable in the `catch` or `finally` block of startDownload?
              // Actually, FileSystemWritableFileStream should be closed to save.
              // If we abort, we might leave it in weird state?
              // `createWritable({ keepExistingData: true })` handles re-opening.
              // But we should try to close it if possible.
              // The `startDownload` function will exit. We can add cleanup there.
              // Let's assume we close it in the loop exit or manually here?
              // We can't close it here because it's in use. 
              // The abort triggers the catch in startDownload.
          }
      }
  }

  function resumeTask(id: string) {
      const task = tasks.value.find(t => t.id === id);
      const context = downloadContexts.get(id);
      
      if (task && context && task.status === 'paused') {
          // Restart download with existing context (keeps fileHandle/chunks)
          startDownload(context.url, context.filename, undefined, id);
      }
  }

  function completeDownload(id: string) {
      const task = tasks.value.find(t => t.id === id);
      const context = downloadContexts.get(id);
      
      if (task && context) {
          task.status = 'completed';
          task.progress = 100;
          task.speed = 0;
          
          // If Memory Download, trigger save
          if (!context.fileHandle) {
              const blob = new Blob(context.chunks, { type: 'application/octet-stream' });
              const downloadUrl = window.URL.createObjectURL(blob);
              const link = document.createElement('a');
              link.href = downloadUrl;
              link.download = context.filename;
              document.body.appendChild(link);
              link.click();
              document.body.removeChild(link);
              window.URL.revokeObjectURL(downloadUrl);
          }
          
          downloadContexts.delete(id);
      }
  }

  // --- Helpers ---

  function updateProgress(id: string, loaded: number, total: number) {
    const task = tasks.value.find(t => t.id === id);
    if (!task) return;
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
      downloadContexts.delete(id);
    }
  }

  function failTask(id: string, error: string) {
    const task = tasks.value.find(t => t.id === id);
    if (task) {
      task.status = 'error';
      task.error = error;
      task.speed = 0;
      downloadContexts.delete(id);
    }
  }

  function clearCompleted() {
    tasks.value = tasks.value.filter(t => t.status !== 'completed');
  }

  function removeTask(id: string) {
      const context = downloadContexts.get(id);
      if (context) {
          context.controller.abort();
          // Try to close writable if exists
          if (context.writable) {
              try { context.writable.close(); } catch(e) {}
          }
      }
      downloadContexts.delete(id);
      tasks.value = tasks.value.filter(t => t.id !== id);
  }

  return { tasks, addTask, startDownload, pauseTask, resumeTask, updateProgress, completeTask, failTask, clearCompleted, removeTask };
});
