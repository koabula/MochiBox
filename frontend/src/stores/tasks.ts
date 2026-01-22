import { defineStore } from 'pinia';
import { ref } from 'vue';
import api from '@/api';

export interface Task {
  id: string;
  type: 'upload' | 'download';
  name: string;
  progress: number;
  speed: number;
  status: 'pending' | 'running' | 'paused' | 'completed' | 'error' | 'canceled';
  phase?: string; // preparing, connecting, downloading
  error?: string;
  loaded: number;
  total: number;
  startTime: number;
}

interface DownloadContext {
  chunks: ArrayBuffer[];
  controller: AbortController;
  url: string;
  filename: string;
  fileHandle?: any;
}

export const useTaskStore = defineStore('tasks', () => {
  const tasks = ref<Task[]>([]);
  
  const downloadContexts = new Map<string, DownloadContext>();
  const backendDownloadPollers = new Map<string, number>();
  const backendDownloadStreams = new Map<string, EventSource>();

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

  function stopBackendPolling(id: string) {
    const timer = backendDownloadPollers.get(id);
    if (timer !== undefined) {
      window.clearInterval(timer);
      backendDownloadPollers.delete(id);
    }
    
    const stream = backendDownloadStreams.get(id);
    if (stream) {
      stream.close();
      backendDownloadStreams.delete(id);
    }
  }

  function applyBackendTaskSnapshot(id: string, dto: any) {
    const task = getTask(id);
    if (!task) return;

    const status = String(dto?.status || '');
    if (status === 'running' || status === 'paused' || status === 'completed' || status === 'error' || status === 'canceled') {
      task.status = status;
    } else {
      task.status = 'error';
      task.error = `Unknown status: ${status}`;
    }

    if (dto?.error) {
      task.error = String(dto.error);
    }

    // Update phase for UI feedback
    task.phase = dto?.phase || '';

    task.loaded = Number(dto?.loaded || 0);
    task.total = Number(dto?.total || 0);
    task.speed = Number(dto?.speed || 0);
    if (task.total > 0) {
      task.progress = Math.min(100, Math.round((task.loaded / task.total) * 100));
    }
  }

  function startBackendPolling(id: string) {
    if (backendDownloadPollers.has(id) || backendDownloadStreams.has(id)) return;
    
    const baseUrl = (api.defaults.baseURL || 'http://localhost:3666/api').replace(/\/api$/, '');
    const streamUrl = `${baseUrl}/api/tasks/download/${id}/stream`;
    
    try {
      const eventSource = new EventSource(streamUrl);
      
      eventSource.onopen = () => {
        console.log(`[SSE] Connection opened for task ${id}`);
      };
      
      eventSource.addEventListener('progress', (event: MessageEvent) => {
        try {
          const dto = JSON.parse(event.data);
          applyBackendTaskSnapshot(id, dto);
          
          if (dto.status === 'completed' || dto.status === 'error' || dto.status === 'canceled') {
            eventSource.close();
            backendDownloadStreams.delete(id);
          }
        } catch (e) {
          console.error(`[SSE] Failed to parse progress event for task ${id}:`, e);
        }
      });
      
      eventSource.onerror = (err) => {
        console.error(`[SSE] Connection error for task ${id}, falling back to polling`, err);
        eventSource.close();
        backendDownloadStreams.delete(id);
        
        const task = getTask(id);
        if (task && task.status !== 'completed' && task.status !== 'error' && task.status !== 'canceled') {
          startBackendPollingFallback(id);
        }
      };
      
      backendDownloadStreams.set(id, eventSource);
    } catch (e) {
      console.error(`[SSE] Failed to create EventSource for task ${id}:`, e);
      startBackendPollingFallback(id);
    }
  }
  
  function startBackendPollingFallback(id: string) {
    if (backendDownloadPollers.has(id)) return;
    const timer = window.setInterval(async () => {
      const task = getTask(id);
      if (!task) {
        stopBackendPolling(id);
        return;
      }
      if (task.status === 'completed' || task.status === 'error' || task.status === 'canceled') {
        stopBackendPolling(id);
        return;
      }
      try {
        const res = await api.get(`/tasks/download/${id}`);
        applyBackendTaskSnapshot(id, res.data);
      } catch (e: any) {
        failTask(id, e?.message || 'Backend task poll failed');
        stopBackendPolling(id);
      }
    }, 500);
    backendDownloadPollers.set(id, timer);
  }

  async function startBackendDownload(
    fileId: number,
    filename: string,
    password?: string,
    encryptionType?: string,
    encryptionMeta?: string,
    cid?: string
  ) {
    const tempId = Date.now().toString() + Math.random().toString().slice(2, 6);
    addTaskWithID(tempId, 'download', filename);
    
    const tempTask = getTask(tempId);
    if (tempTask) {
      tempTask.status = 'pending';
    }

    try {
      const payload: any = {
        password: password || '',
      };
      
      if (fileId > 0) {
        payload.file_id = fileId;
      } else if (cid) {
        payload.cid = cid;
        payload.name = filename;
        if (encryptionType) payload.encryption_type = encryptionType;
        if (encryptionMeta) payload.encryption_meta = encryptionMeta;
      } else {
        removeTask(tempId);
        throw new Error('Either fileId or cid must be provided');
      }

      const res = await api.post('/tasks/download/start', payload);
      const dto = res.data as any;
      const realId = String(dto?.id || '');
      
      if (!realId) {
        removeTask(tempId);
        throw new Error('Backend did not return task id');
      }

      const index = tasks.value.findIndex(t => t.id === tempId);
      if (index !== -1) {
        tasks.value[index].id = realId;
        applyBackendTaskSnapshot(realId, dto);
        startBackendPolling(realId);
      }

      return realId;
    } catch (e: any) {
      removeTask(tempId);
      throw e;
    }
  }

  async function startDownload(url: string, filename: string, fileHandle?: any, taskId?: string) {
    let id = taskId;
    if (!id) {
       id = addTask('download', filename);
    }
    
    const task = tasks.value.find(t => t.id === id);
    if (!task) return;

    task.status = 'running';
    if (!task.startTime) task.startTime = Date.now();
    
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
    if (fileHandle) {
        context.fileHandle = fileHandle;
    }

    let writable: any = null;
    try {
        const fullUrl = url.startsWith('http') ? url : `${api.defaults.baseURL}${url}`;
        
        const headers: HeadersInit = {};
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
        const total = contentLength ? parseInt(contentLength, 10) + task.loaded : 0;
        if (task.total === 0 && total > 0) {
            task.total = total;
        }

        const reader = response.body?.getReader();
        if (!reader) throw new Error('ReadableStream not supported');

        const progressIntervalMs = 200;
        const speedIntervalMs = 500;
        const now0 = performance.now();
        let lastProgressAt = now0;
        let lastSpeedAt = now0;
        let lastSpeedLoaded = task.loaded;
        let loadedLocal = task.loaded;

        if (context.fileHandle) {
            writable = await context.fileHandle.createWritable({ keepExistingData: true });
            await writable.seek(task.loaded);
        }
        
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            
            if (value) {
                if (writable) {
                    await writable.write(value);
                } else {
                    context.chunks.push(value.buffer.slice(value.byteOffset, value.byteOffset + value.byteLength));
                }

                loadedLocal += value.length;

                const now = performance.now();
                if (now - lastSpeedAt >= speedIntervalMs) {
                    const dt = (now - lastSpeedAt) / 1000;
                    const dl = loadedLocal - lastSpeedLoaded;
                    const instant = dt > 0 ? dl / dt : 0;
                    task.speed = task.speed === 0 ? instant : (task.speed * 0.7 + instant * 0.3);
                    lastSpeedAt = now;
                    lastSpeedLoaded = loadedLocal;
                }

                if (now - lastProgressAt >= progressIntervalMs) {
                    updateProgress(id!, loadedLocal, task.total);
                    lastProgressAt = now;
                }
            }
        }

        updateProgress(id!, loadedLocal, task.total);
        if (writable) {
            await writable.close();
            writable = null;
        }
        completeDownload(id!);

    } catch (err: any) {
        if (err.name === 'AbortError') {
            return;
        } else {
            failTask(id!, err.message);
        }
    } finally {
        if (writable) {
            try {
                await writable.close();
            } catch (_) {}
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
          return;
      }

      if (task && task.type === 'download' && task.status === 'running') {
        api.post(`/tasks/download/${id}/pause`).then((res) => {
          applyBackendTaskSnapshot(id, res.data);
          stopBackendPolling(id);
        }).catch((e: any) => {
          failTask(id, e?.message || 'Pause failed');
        });
      }
  }

  function resumeTask(id: string) {
      const task = tasks.value.find(t => t.id === id);
      const context = downloadContexts.get(id);
      
      if (task && context && task.status === 'paused') {
          startDownload(context.url, context.filename, undefined, id);
          return;
      }

      if (task && task.type === 'download' && (task.status === 'paused' || task.status === 'error')) {
        api.post(`/tasks/download/${id}/resume`).then((res) => {
          applyBackendTaskSnapshot(id, res.data);
          startBackendPolling(id);
        }).catch((e: any) => {
          failTask(id, e?.message || 'Resume failed');
        });
      }
  }

  function completeDownload(id: string) {
      const task = tasks.value.find(t => t.id === id);
      const context = downloadContexts.get(id);
      
      if (task && context) {
          task.status = 'completed';
          task.progress = 100;
          task.speed = 0;
          
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
      stopBackendPolling(id);
    }
  }

  function failTask(id: string, error: string) {
    const task = tasks.value.find(t => t.id === id);
    if (task) {
      task.status = 'error';
      task.error = error;
      task.speed = 0;
      downloadContexts.delete(id);
      stopBackendPolling(id);
    }
  }

  function clearCompleted() {
    tasks.value = tasks.value.filter(t => t.status !== 'completed' && t.status !== 'canceled');
  }

  function removeTask(id: string) {
      const context = downloadContexts.get(id);
      if (context) {
          context.controller.abort();
      }
      downloadContexts.delete(id);
      stopBackendPolling(id);
      api.post(`/tasks/download/${id}/cancel`).catch(() => {});
      tasks.value = tasks.value.filter(t => t.id !== id);
  }

  return { 
    tasks, 
    addTask, 
    startBackendDownload, 
    startDownload, 
    pauseTask, 
    resumeTask, 
    updateProgress, 
    completeTask, 
    failTask, 
    clearCompleted, 
    removeTask 
  };
});
