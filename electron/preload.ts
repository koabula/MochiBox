import { contextBridge, ipcRenderer, clipboard } from 'electron';

contextBridge.exposeInMainWorld('electronAPI', {
  minimize: () => ipcRenderer.send('window-minimize'),
  maximize: () => ipcRenderer.send('window-maximize'),
  close: () => ipcRenderer.send('window-close'),
  restart: () => ipcRenderer.send('app-restart'),
  clipboard: {
    writeText: (text: string) => clipboard.writeText(text),
    readText: () => clipboard.readText(),
  },
});

// Enhanced electron API for stream download support
contextBridge.exposeInMainWorld('electron', {
  appendFile: async (filename: string, data: Uint8Array): Promise<void> => {
    return ipcRenderer.invoke('append-file', filename, data);
  },
});
