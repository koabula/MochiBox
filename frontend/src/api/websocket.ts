import { ref, onUnmounted } from 'vue';
import api from './index';

interface WSMessage {
  task_id: string;
  type: string;
  data: any;
}

class TaskWebSocket {
  private ws: WebSocket | null = null;
  private reconnectTimer: number | null = null;
  private subscriptions = new Map<string, Set<(data: any) => void>>();
  private connected = ref(false);
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;

  connect() {
    if (this.ws?.readyState === WebSocket.OPEN) {
      return;
    }

    const baseURL = api.defaults.baseURL || 'http://localhost:3666';
    const wsURL = baseURL.replace(/^http/, 'ws') + '/api/ws/tasks';

    try {
      this.ws = new WebSocket(wsURL);

      this.ws.onopen = () => {
        console.log('WebSocket connected');
        this.connected.value = true;
        this.reconnectAttempts = 0;

        this.subscriptions.forEach((_, taskId) => {
          this.sendSubscribe(taskId);
        });
      };

      this.ws.onmessage = (event) => {
        try {
          const message: WSMessage = JSON.parse(event.data);
          if (message.type === 'task_update' && message.task_id) {
            const handlers = this.subscriptions.get(message.task_id);
            if (handlers) {
              handlers.forEach(handler => handler(message.data));
            }
          }
        } catch (e) {
          console.error('Failed to parse WebSocket message:', e);
        }
      };

      this.ws.onerror = (error) => {
        console.error('WebSocket error:', error);
      };

      this.ws.onclose = () => {
        console.log('WebSocket disconnected');
        this.connected.value = false;
        this.scheduleReconnect();
      };
    } catch (error) {
      console.error('Failed to create WebSocket:', error);
      this.scheduleReconnect();
    }
  }

  private scheduleReconnect() {
    if (this.reconnectTimer) {
      return;
    }

    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.log('Max reconnect attempts reached');
      return;
    }

    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
    this.reconnectAttempts++;

    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, delay);
  }

  subscribe(taskId: string, callback: (data: any) => void) {
    if (!this.subscriptions.has(taskId)) {
      this.subscriptions.set(taskId, new Set());
    }
    this.subscriptions.get(taskId)!.add(callback);

    if (this.ws?.readyState === WebSocket.OPEN) {
      this.sendSubscribe(taskId);
    }
  }

  unsubscribe(taskId: string, callback?: (data: any) => void) {
    if (callback) {
      const handlers = this.subscriptions.get(taskId);
      if (handlers) {
        handlers.delete(callback);
        if (handlers.size === 0) {
          this.subscriptions.delete(taskId);
          this.sendUnsubscribe(taskId);
        }
      }
    } else {
      this.subscriptions.delete(taskId);
      this.sendUnsubscribe(taskId);
    }
  }

  private sendSubscribe(taskId: string) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ action: 'subscribe', task_id: taskId }));
    }
  }

  private sendUnsubscribe(taskId: string) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ action: 'unsubscribe', task_id: taskId }));
    }
  }

  disconnect() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }

    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }

    this.subscriptions.clear();
    this.connected.value = false;
  }

  isConnected() {
    return this.connected.value;
  }
}

export const taskWebSocket = new TaskWebSocket();

export function useTaskWebSocket(taskId: string, onUpdate: (data: any) => void) {
  taskWebSocket.subscribe(taskId, onUpdate);

  onUnmounted(() => {
    taskWebSocket.unsubscribe(taskId, onUpdate);
  });
}
