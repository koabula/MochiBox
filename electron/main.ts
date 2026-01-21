import { app, BrowserWindow, ipcMain, dialog, shell } from 'electron';
import path from 'path';
import { spawn, ChildProcess } from 'child_process';
import fs from 'fs';
import http from 'http';

let mainWindow: BrowserWindow | null = null;
let backendProcess: ChildProcess | null = null;
let isQuitting = false;

// const isDev = process.env.NODE_ENV === 'development';
const isDev = process.env.NODE_ENV === 'development' || (!app.isPackaged && process.env.NODE_ENV !== 'production');

app.setName('MochiBox');
app.setAppUserModelId('com.mochibox.desktop');

function logToFile(msg: string) {
  try {
    const logPath = path.join(app.getPath('userData'), 'electron-main.log');
    fs.appendFileSync(logPath, `[${new Date().toISOString()}] ${msg}\n`);
  } catch (e) {
    console.error('Failed to write to log file:', e);
  }
}

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
  ipcMain.on('app-restart', () => {
    app.relaunch();
    app.exit(0);
  });

  // IPC Handler for stream download file append
  ipcMain.handle('append-file', async (event, filename: string, data: Uint8Array) => {
    try {
      const buffer = Buffer.from(data);
      await fs.promises.appendFile(filename, buffer);
    } catch (error) {
      console.error('Failed to append file:', error);
      throw error;
    }
  });

  // Handle external links
  mainWindow.webContents.setWindowOpenHandler((details) => {
    if (details.url.startsWith('http:') || details.url.startsWith('https:')) {
      shell.openExternal(details.url);
      return { action: 'deny' };
    }
    return { action: 'allow' };
  });

  // Debugging hooks
  mainWindow.webContents.on('did-fail-load', (event, errorCode, errorDescription) => {
    console.error(`Page failed to load: ${errorCode} - ${errorDescription}`);
  });
  
  if (isDev) {
      mainWindow.loadURL('http://localhost:5173');
      mainWindow.webContents.openDevTools();
    } else {
      const indexPath = path.join(__dirname, '../frontend/dist/index.html');
      console.log("Loading index from:", indexPath);
      mainWindow.loadFile(indexPath).catch(e => console.error("Failed to load index.html:", e));
    }

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

function startBackend() {
  if (backendProcess) return;

  const binaryName = process.platform === 'win32' ? 'mochibox-core.exe' : 'mochibox-core';

  const backendPath = isDev 
    ? path.join(__dirname, `../../backend/${binaryName}`) // Assuming we build it here for dev
    : path.join(process.resourcesPath, `bin/${binaryName}`);

  // Calculate IPFS binary path
  const ipfsName = process.platform === 'win32' ? 'ipfs.exe' : 'ipfs';
  const ipfsPath = isDev
    ? path.join(__dirname, `../../electron/resources/bin/${process.platform === 'win32' ? 'win' : process.platform === 'darwin' ? 'mac' : 'linux'}/${ipfsName}`)
    : path.join(process.resourcesPath, `bin/${ipfsName}`);

  console.log('Starting backend from:', backendPath);
  console.log('Using IPFS binary from:', ipfsPath);
  console.log('Backend CWD:', path.dirname(backendPath));
  console.log('Resources Path:', process.resourcesPath);

  logToFile(`Starting backend from: ${backendPath}`);
  logToFile(`Using IPFS binary from: ${ipfsPath}`);
  logToFile(`Resources Path: ${process.resourcesPath}`);
  logToFile(`Backend CWD: ${app.getPath('userData')}`);

  if (fs.existsSync(backendPath)) {
      // Ensure we check if IPFS binary exists too
      let env: NodeJS.ProcessEnv = { ...process.env, MOCHIBOX_PORT: '3666' };

      if (fs.existsSync(ipfsPath)) {
          env.MOCHIBOX_IPFS_BIN = ipfsPath;
      } else {
          console.warn(`[Main] IPFS binary not found at calculated path: ${ipfsPath}. Backend will use fallback.`);
          logToFile(`[Main] WARNING: IPFS binary not found at calculated path: ${ipfsPath}. Backend will use fallback.`);
      }

      try {
        backendProcess = spawn(backendPath, [], {
          cwd: app.getPath('userData'), // Change CWD to userData to avoid permissions issues
          env: env
        });

        backendProcess.stdout?.on('data', (data) => {
          console.log(`[Go]: ${data}`);
          logToFile(`[Go]: ${data}`);
        });

        backendProcess.stderr?.on('data', (data) => {
          console.error(`[Go Error]: ${data}`);
          logToFile(`[Go Error]: ${data}`);
        });
        
        backendProcess.on('error', (err) => {
          console.error('Failed to start backend process:', err);
          logToFile(`Failed to start backend process: ${err.message}`);
          if (!isDev) {
            dialog.showErrorBox('MochiBox Backend Error', `Failed to start backend process: ${err.message}`);
          }
        });

        backendProcess.on('close', (code, signal) => {
            console.log(`Backend process exited with code ${code}, signal ${signal ?? 'none'}`);
            logToFile(`Backend process exited with code ${code}, signal ${signal ?? 'none'}`);
            if (!isDev && !isQuitting && typeof code === 'number' && code !== 0) {
              dialog.showErrorBox('MochiBox Backend Exit', `Backend process exited with code ${code}`);
            }
            backendProcess = null;
        });
      } catch (err) {
        logToFile(`Exception spawning backend: ${err}`);
      }
  } else {
      console.warn("Backend binary not found at", backendPath);
      logToFile(`Backend binary not found at ${backendPath}`);
  }
}

async function stopBackend() {
  if (backendProcess) {
    console.log('Stopping backend...');
    logToFile('Stopping backend...');
    
    // Attempt graceful shutdown via Stdin
    if (backendProcess.stdin) {
        try {
            backendProcess.stdin.write('shutdown\n');
            backendProcess.stdin.end(); // Close stream to trigger EOF on Go side
            logToFile('Sent shutdown signal to backend stdin');
        } catch (err) {
            logToFile(`Failed to write to stdin: ${err}`);
        }
    }

    // Wait for process to exit gracefully
    const exitPromise = new Promise<void>((resolve) => {
        if (!backendProcess) {
            resolve();
            return;
        }
        
        const timeout = setTimeout(() => {
            resolve();
        }, 5000); // Wait up to 5 seconds

        backendProcess.once('close', () => {
            clearTimeout(timeout);
            resolve();
        });
    });

    await exitPromise;

    if (backendProcess) {
      console.log('Backend did not exit in time, forcing kill...');
      logToFile('Backend did not exit in time, forcing kill...');
      backendProcess.kill();
      backendProcess = null;
    } else {
        logToFile('Backend exited gracefully.');
    }
  }
}

app.on('ready', () => {
  startBackend(); // TODO: Enable when Go binary is ready
  createWindow();
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('will-quit', async (e) => {
  if (backendProcess) {
    e.preventDefault();
    isQuitting = true;
    await stopBackend();
    app.quit();
  }
});

app.on('activate', () => {
  if (mainWindow === null) {
    createWindow();
  }
});
