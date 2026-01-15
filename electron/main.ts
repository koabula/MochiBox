import { app, BrowserWindow, ipcMain } from 'electron';
import path from 'path';
import { spawn, ChildProcess } from 'child_process';
import fs from 'fs';

let mainWindow: BrowserWindow | null = null;
let backendProcess: ChildProcess | null = null;

// const isDev = process.env.NODE_ENV === 'development';
const isDev = process.env.NODE_ENV === 'development' || (!app.isPackaged && process.env.NODE_ENV !== 'production');

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    minWidth: 800,
    minHeight: 600,
    frame: false, // Custom frame
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      nodeIntegration: false,
      contextIsolation: true,
    },
    icon: path.join(__dirname, '../../icon.png'),
  });

  // IPC Handlers for Window Controls
  ipcMain.on('window-minimize', () => mainWindow?.minimize());
  ipcMain.on('window-maximize', () => {
    if (mainWindow?.isMaximized()) {
      mainWindow.unmaximize();
    } else {
      mainWindow?.maximize();
    }
  });
  ipcMain.on('window-close', () => mainWindow?.close());

  if (isDev) {
    mainWindow.loadURL('http://localhost:5173');
    mainWindow.webContents.openDevTools();
  } else {
    mainWindow.loadFile(path.join(__dirname, '../frontend/dist/index.html'));
  }

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

function startBackend() {
  if (backendProcess) return;

  const backendPath = isDev 
    ? path.join(__dirname, '../../backend/mochibox-core.exe') // Assuming we build it here for dev
    : path.join(process.resourcesPath, 'bin/mochibox-core.exe');

  console.log('Starting backend from:', backendPath);

  if (fs.existsSync(backendPath)) {
      backendProcess = spawn(backendPath, [], {
        cwd: path.dirname(backendPath),
        env: { ...process.env, MOCHIBOX_PORT: '3666' }
      });

      backendProcess.stdout?.on('data', (data) => {
        console.log(`[Go]: ${data}`);
      });

      backendProcess.stderr?.on('data', (data) => {
        console.error(`[Go Error]: ${data}`);
      });
      
      backendProcess.on('close', (code) => {
          console.log(`Backend process exited with code ${code}`);
          backendProcess = null;
      });
  } else {
      console.warn("Backend binary not found at", backendPath);
  }
}

function stopBackend() {
  if (backendProcess) {
    backendProcess.kill();
    backendProcess = null;
  }
}

app.on('ready', () => {
  // startBackend(); // TODO: Enable when Go binary is ready
  createWindow();
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('will-quit', () => {
  stopBackend();
});

app.on('activate', () => {
  if (mainWindow === null) {
    createWindow();
  }
});
