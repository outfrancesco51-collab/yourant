/**
 * Yourant Desktop - Main Application Process
 *
 * Manages the Electron lifecycle, sidecar Go server process, health check polling,
 * frameless window creation, privileged custom protocol (app://) with COOP/COEP headers,
 * IPC handlers for window controls, and CastSender for Chromecast LAN streaming.
 */

const { app, BrowserWindow, protocol, ipcMain, net, clipboard } = require("electron");
const path = require("path");
const fs = require("fs");
const { spawn } = require("child_process");
const http = require("http");
const { pathToFileURL } = require("url");
const { CastSender } = require("./sender");

// Server configuration constants
const SERVER_PORT = 8080;
const SERVER_HOST = "127.0.0.1";
const isDev = !app.isPackaged || process.env.NODE_ENV === "development";

// Global references
let mainWindow = null;
let serverProcess = null;
let castSender = null;
let isQuitting = false;

// 1. Register Privileged Schemes (MUST be called before app.whenReady)
protocol.registerSchemesAsPrivileged([
  {
    scheme: "app",
    privileges: {
      standard: true,
      secure: true,
      allowServiceWorkers: true,
      supportFetchAPI: true,
      corsEnabled: true,
      stream: true,
    },
  },
]);

/**
 * Resolves directory containing built frontend assets (frontend/dist).
 */
function getFrontendDistPath() {
  const candidates = [
    path.join(__dirname, "web"),
    path.join(__dirname, "../frontend/dist"),
    path.join(app.getAppPath(), "frontend/dist"),
    path.join(app.getAppPath(), "web"),
    path.join(process.resourcesPath, "frontend/dist"),
    path.join(process.resourcesPath, "web"),
    path.join(process.resourcesPath, "app.asar.unpacked/frontend/dist"),
  ];

  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) {
      return path.resolve(candidate);
    }
  }
  return path.resolve(__dirname, "../frontend/dist");
}

/**
 * Resolves platform-specific Go backend server binary path.
 */
function resolveGoBinaryPath() {
  const isWin = process.platform === "win32";
  const isMac = process.platform === "darwin";
  const arch = process.arch === "arm64" ? "arm64" : "amd64";

  const targetSuffix = isWin ? "windows-amd64.exe" : isMac ? `darwin-${arch}` : `linux-${arch}`;
  const candidates = [
    `yourant-server-${targetSuffix}`,
    isWin ? "yourant-server.exe" : "yourant-server",
    isWin ? "server.exe" : "server",
  ];

  const candidateDirs = [
    path.join(process.resourcesPath, "binaries"),
    path.join(app.getAppPath(), "binaries"),
    path.join(__dirname, "binaries"),
    path.join(__dirname, "../binaries"),
  ];

  for (const dir of candidateDirs) {
    for (const name of candidates) {
      const fullPath = path.join(dir, name);
      if (fs.existsSync(fullPath)) {
        return fullPath;
      }
    }
  }
  return null;
}

/**
 * Checks if the Go backend health endpoint responds with HTTP 200 and status: ok.
 */
function checkServerHealth(port = SERVER_PORT) {
  return new Promise((resolve) => {
    const req = http.get(
      `http://${SERVER_HOST}:${port}/api/health`,
      { timeout: 1500 },
      (res) => {
        let body = "";
        res.on("data", (chunk) => {
          body += chunk;
        });
        res.on("end", () => {
          try {
            const data = JSON.parse(body);
            resolve(res.statusCode === 200 && data.status === "ok");
          } catch (_) {
            resolve(res.statusCode === 200);
          }
        });
      }
    );

    req.on("error", () => resolve(false));
    req.on("timeout", () => {
      req.destroy();
      resolve(false);
    });
  });
}

/**
 * Polls the Go server health check until ready (every 300ms, up to 30 retries).
 */
async function waitForServer(port = SERVER_PORT, maxRetries = 30, intervalMs = 300) {
  console.log(`[Main] Polling http://${SERVER_HOST}:${port}/api/health for readiness...`);
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    const isHealthy = await checkServerHealth(port);
    if (isHealthy) {
      console.log(`[Main] Go backend is healthy (attempt ${attempt}/${maxRetries})`);
      return true;
    }
    await new Promise((resolve) => setTimeout(resolve, intervalMs));
  }
  console.warn(`[Main] Health probe timed out after ${maxRetries * intervalMs}ms`);
  return false;
}

/**
 * Spawns or connects to the Go server sidecar process.
 */
async function startGoServer() {
  // First, verify if server is already running (e.g. independently in dev)
  const alreadyRunning = await checkServerHealth(SERVER_PORT);
  if (alreadyRunning) {
    console.log(`[Main] Go server already running on port ${SERVER_PORT}`);
    return;
  }

  const binaryPath = resolveGoBinaryPath();
  const repoRoot = path.resolve(__dirname, "..");
  const userDataDir = app.getPath("userData");
  const downloadsDir = path.join(userDataDir, "downloads");
  const dataDir = path.join(userDataDir, "data");
  const storageFile = path.join(dataDir, "library.json");

  // Ensure storage directories exist
  try {
    fs.mkdirSync(downloadsDir, { recursive: true });
    fs.mkdirSync(dataDir, { recursive: true });
  } catch (_) {}

  const serverArgs = [
    "-port",
    String(SERVER_PORT),
    "-host",
    SERVER_HOST,
    "-downloads-dir",
    downloadsDir,
    "-storage-file",
    storageFile,
    "-static-dir",
    getFrontendDistPath(),
  ];

  if (binaryPath) {
    console.log(`[Main] Spawning sidecar Go server binary: ${binaryPath}`);
    if (process.platform !== "win32") {
      try {
        fs.chmodSync(binaryPath, 0o755);
      } catch (_) {}
    }

    serverProcess = spawn(binaryPath, serverArgs, {
      stdio: ["ignore", "pipe", "pipe"],
      windowsHide: true,
      env: { ...process.env, PORT: String(SERVER_PORT), HOST: SERVER_HOST },
    });
  } else if (isDev) {
    // In development without pre-compiled binary, fallback to `go run ./cmd/server`
    console.log("[Main] Development mode: Spawning Go server via `go run ./cmd/server`...");
    serverProcess = spawn("go", ["run", "./cmd/server", ...serverArgs], {
      cwd: repoRoot,
      stdio: ["ignore", "pipe", "pipe"],
      windowsHide: true,
      env: { ...process.env, PORT: String(SERVER_PORT), HOST: SERVER_HOST },
    });
  } else {
    console.error("[Main] Error: Could not resolve Go backend binary in production!");
    return;
  }

  if (serverProcess) {
    serverProcess.stdout.on("data", (data) => {
      console.log(`[Server] ${data.toString().trim()}`);
    });

    serverProcess.stderr.on("data", (data) => {
      console.error(`[Server ERR] ${data.toString().trim()}`);
    });

    serverProcess.on("error", (err) => {
      console.error("[Main] Go server process error:", err);
    });

    serverProcess.on("exit", (code, signal) => {
      console.log(`[Main] Go server process exited with code ${code}, signal ${signal}`);
      serverProcess = null;
      if (!isQuitting && mainWindow && !mainWindow.isDestroyed()) {
        mainWindow.webContents.send("server-status", { running: false, code, signal });
      }
    });
  }
}

/**
 * Gracefully terminates the Go backend sidecar child process.
 */
function terminateServerProcess() {
  if (!serverProcess || serverProcess.killed) return;
  console.log(`[Main] Terminating Go backend sidecar (PID: ${serverProcess.pid})...`);

  try {
    if (process.platform === "win32") {
      serverProcess.kill("SIGINT");
      setTimeout(() => {
        if (serverProcess && !serverProcess.killed) {
          try {
            spawn("taskkill", ["/pid", String(serverProcess.pid), "/T", "/F"]);
          } catch (_) {}
        }
      }, 3000);
    } else {
      serverProcess.kill("SIGTERM");
      setTimeout(() => {
        if (serverProcess && !serverProcess.killed) {
          serverProcess.kill("SIGKILL");
        }
      }, 3000);
    }
  } catch (err) {
    console.error("[Main] Error during Go server teardown:", err);
  }
}

/**
 * Registers custom protocol 'app://' serving static frontend assets
 * with COOP/COEP headers mandatory for Web Worker SharedArrayBuffer/WASM.
 */
function setupCustomProtocol() {
  protocol.handle("app", async (request) => {
    try {
      const requestUrl = new URL(request.url);
      let urlPath = decodeURIComponent(requestUrl.pathname);
      const distPath = getFrontendDistPath();

      if (!urlPath || urlPath === "/" || urlPath === "/-") {
        urlPath = "/index.html";
      }

      let filePath = path.join(distPath, urlPath);
      const resolvedPath = path.resolve(filePath);

      // Traversal protection
      if (!resolvedPath.startsWith(distPath)) {
        filePath = path.join(distPath, "index.html");
      }

      if (fs.existsSync(filePath) && fs.statSync(filePath).isFile()) {
        const response = await net.fetch(pathToFileURL(filePath).toString());
        const headers = new Headers(response.headers);
        headers.set("Cross-Origin-Opener-Policy", "same-origin");
        headers.set("Cross-Origin-Embedder-Policy", "credentialless");
        return new Response(response.body, {
          status: response.status,
          statusText: response.statusText,
          headers,
        });
      }

      // SPA client routing fallback to index.html
      const ext = path.extname(urlPath);
      if (!ext || ext === ".html") {
        const fallbackPath = path.join(distPath, "index.html");
        if (fs.existsSync(fallbackPath) && fs.statSync(fallbackPath).isFile()) {
          const response = await net.fetch(pathToFileURL(fallbackPath).toString());
          const headers = new Headers(response.headers);
          headers.set("Cross-Origin-Opener-Policy", "same-origin");
          headers.set("Cross-Origin-Embedder-Policy", "credentialless");
          return new Response(response.body, {
            status: 200,
            statusText: "OK",
            headers,
          });
        }
      }

      return new Response("Not Found", { status: 404 });
    } catch (err) {
      console.error("[Main] Custom protocol error:", err);
      return new Response("Internal Server Error", { status: 500 });
    }
  });
}

/**
 * Initializes and wires CastSender instance events to the renderer.
 */
function ensureCastSender() {
  if (!castSender) {
    castSender = new CastSender();

    castSender.on("deviceFound", (device) => {
      if (mainWindow && !mainWindow.isDestroyed()) {
        mainWindow.webContents.send("cast:deviceFound", device);
      }
    });

    castSender.on("sessionUpdate", (session) => {
      if (mainWindow && !mainWindow.isDestroyed()) {
        mainWindow.webContents.send("cast:sessionUpdate", session);
      }
    });

    castSender.on("mediaStatus", (status) => {
      if (mainWindow && !mainWindow.isDestroyed()) {
        mainWindow.webContents.send("cast:mediaStatus", status);
      }
    });

    castSender.on("receiverReady", () => {
      if (mainWindow && !mainWindow.isDestroyed()) {
        mainWindow.webContents.send("cast:receiverReady");
      }
    });

    castSender.on("error", (err) => {
      if (mainWindow && !mainWindow.isDestroyed()) {
        mainWindow.webContents.send("cast:error", err);
      }
    });
  }
  return castSender;
}

/**
 * Registers all IPC handlers for window controls, server status, and casting.
 */
function registerIpcHandlers() {
  // Window Controls
  ipcMain.on("window:minimize", () => {
    if (mainWindow && !mainWindow.isDestroyed()) mainWindow.minimize();
  });

  ipcMain.on("window:maximize", () => {
    if (mainWindow && !mainWindow.isDestroyed()) {
      if (mainWindow.isMaximized()) mainWindow.unmaximize();
      else mainWindow.maximize();
    }
  });

  ipcMain.on("window:close", () => {
    if (mainWindow && !mainWindow.isDestroyed()) mainWindow.close();
  });

  ipcMain.on("window:toggleMaximize", () => {
    if (mainWindow && !mainWindow.isDestroyed()) {
      if (mainWindow.isMaximized()) mainWindow.unmaximize();
      else mainWindow.maximize();
    }
  });

  ipcMain.handle("window:isMaximized", () => {
    return mainWindow && !mainWindow.isDestroyed() ? mainWindow.isMaximized() : false;
  });

  ipcMain.handle("window:isMinimizable", () => {
    return mainWindow && !mainWindow.isDestroyed() ? mainWindow.minimizable : false;
  });

  ipcMain.handle("window:isMaximizable", () => {
    return mainWindow && !mainWindow.isDestroyed() ? mainWindow.maximizable : false;
  });

  ipcMain.handle("window:isClosable", () => {
    return mainWindow && !mainWindow.isDestroyed() ? mainWindow.closable : false;
  });

  ipcMain.handle("window:isFullscreen", () => {
    return mainWindow && !mainWindow.isDestroyed() ? mainWindow.isFullScreen() : false;
  });

  ipcMain.on("window:setFullscreen", (_, fullscreen) => {
    if (mainWindow && !mainWindow.isDestroyed()) mainWindow.setFullScreen(Boolean(fullscreen));
  });

  ipcMain.on("window:hide", () => {
    if (mainWindow && !mainWindow.isDestroyed()) mainWindow.hide();
  });

  ipcMain.on("window:show", () => {
    if (mainWindow && !mainWindow.isDestroyed()) mainWindow.show();
  });

  // Local Go Backend Status & Lifecycle
  ipcMain.handle("get-local-server-port", () => SERVER_PORT);

  ipcMain.handle("is-local-server-reachable", async () => {
    return await checkServerHealth(SERVER_PORT);
  });

  ipcMain.handle("restart-server", async () => {
    terminateServerProcess();
    await startGoServer();
    return await waitForServer(SERVER_PORT, 15, 300);
  });

  ipcMain.handle("kill-server", () => {
    terminateServerProcess();
    return true;
  });

  ipcMain.on("restart-server", async () => {
    terminateServerProcess();
    await startGoServer();
  });

  ipcMain.on("kill-server", () => {
    terminateServerProcess();
  });

  ipcMain.on("restart-app", () => {
    app.relaunch();
    app.quit();
  });

  ipcMain.on("quit-app", () => {
    app.quit();
  });

  ipcMain.handle("clipboard:writeText", (_, text) => {
    clipboard.writeText(String(text || ""));
    return true;
  });

  // Cast IPC Handlers
  ipcMain.handle("cast:discover", async () => {
    const sender = ensureCastSender();
    sender.startDiscovery();
  });

  ipcMain.handle("cast:stopDiscovery", async () => {
    if (castSender) castSender.stopDiscovery();
  });

  ipcMain.handle("cast:getDevices", async () => {
    return castSender ? castSender.getDevices() : [];
  });

  ipcMain.handle("cast:connect", async (_, deviceId) => {
    const sender = ensureCastSender();
    return await sender.connect(deviceId);
  });

  ipcMain.handle("cast:disconnect", async () => {
    if (castSender) castSender.disconnect();
  });

  ipcMain.handle("cast:getStatus", async () => {
    if (!castSender) {
      return { connected: false, device: null, sessionId: null, mediaStatus: null };
    }
    return castSender.getStatus();
  });

  ipcMain.handle("cast:loadMedia", async (_, opts) => {
    const sender = ensureCastSender();
    return sender.loadMedia({ ...opts, serverPort: SERVER_PORT });
  });

  ipcMain.handle("cast:play", async () => {
    if (castSender) castSender.play();
  });

  ipcMain.handle("cast:pause", async () => {
    if (castSender) castSender.pause();
  });

  ipcMain.handle("cast:seek", async (_, time) => {
    if (castSender) castSender.seek(time);
  });

  ipcMain.handle("cast:stop", async () => {
    if (castSender) castSender.stop();
  });

  ipcMain.handle("cast:setVolume", async (_, level) => {
    if (castSender) castSender.setVolume(level);
  });

  ipcMain.handle("cast:setMuted", async (_, muted) => {
    if (castSender) castSender.setMuted(muted);
  });

  ipcMain.handle("cast:sendSubtitleEvents", async (_, events) => {
    if (castSender) castSender.sendSubtitleEvents(events);
  });

  ipcMain.handle("cast:sendSubtitleTracks", async (_, tracks) => {
    if (castSender) castSender.sendSubtitleTracks(tracks);
  });

  ipcMain.handle("cast:switchSubtitleTrack", async (_, trackNumber) => {
    if (castSender) castSender.switchSubtitleTrack(trackNumber);
  });

  ipcMain.handle("cast:sendFonts", async (_, fontUrls, serverPort) => {
    if (castSender) castSender.sendFonts(fontUrls, serverPort || SERVER_PORT);
  });

  ipcMain.handle("cast:sendSubtitleHeader", async (_, header) => {
    if (castSender) castSender.sendSubtitleHeader(header);
  });

  ipcMain.handle("cast:disableSubtitles", async () => {
    if (castSender) castSender.disableSubtitles();
  });

  ipcMain.handle("cast:getLanIP", async () => {
    const sender = ensureCastSender();
    return sender.getLanIP();
  });
}

/**
 * Creates the primary frameless application window.
 */
function createMainWindow() {
  mainWindow = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 960,
    minHeight: 600,
    backgroundColor: "#070a13", // Yourant metallic obsidian background
    frame: false,
    titleBarStyle: process.platform === "darwin" ? "hiddenInset" : "hidden",
    titleBarOverlay: false,
    show: false,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      sandbox: false,
      preload: path.join(__dirname, "preload.js"),
    },
  });

  // Window state notification forwarders
  mainWindow.on("maximize", () => {
    if (!mainWindow.isDestroyed()) mainWindow.webContents.send("window:maximized");
  });

  mainWindow.on("unmaximize", () => {
    if (!mainWindow.isDestroyed()) mainWindow.webContents.send("window:unmaximized");
  });

  mainWindow.on("minimize", () => {
    if (!mainWindow.isDestroyed()) mainWindow.webContents.send("window:minimized");
  });

  mainWindow.on("enter-full-screen", () => {
    if (!mainWindow.isDestroyed()) mainWindow.webContents.send("window:fullscreen", true);
  });

  mainWindow.on("leave-full-screen", () => {
    if (!mainWindow.isDestroyed()) mainWindow.webContents.send("window:fullscreen", false);
  });

  mainWindow.once("ready-to-show", () => {
    mainWindow.show();
    mainWindow.focus();
  });

  mainWindow.on("closed", () => {
    mainWindow = null;
  });

  // Load renderer: Vite dev server in development if running, else custom protocol app://
  if (isDev && process.env.VITE_DEV_SERVER_URL) {
    mainWindow.loadURL(process.env.VITE_DEV_SERVER_URL);
  } else {
    mainWindow.loadURL("app://index.html");
  }
}

// Enforce single instance lock
const gotSingleInstanceLock = app.requestSingleInstanceLock();
if (!gotSingleInstanceLock) {
  console.log("[Main] Another instance is already running. Quitting.");
  app.quit();
} else {
  app.on("second-instance", () => {
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      mainWindow.focus();
    }
  });

  // Application Lifecycle Management
  app.whenReady().then(async () => {
    console.log("[Main] App ready. Initializing custom protocol & sidecar...");
    setupCustomProtocol();
    registerIpcHandlers();

    // Start sidecar Go backend and poll health check
    await startGoServer();
    await waitForServer(SERVER_PORT, 30, 300);

    createMainWindow();

    app.on("activate", () => {
      if (BrowserWindow.getAllWindows().length === 0) {
        createMainWindow();
      }
    });
  });

  app.on("window-all-closed", () => {
    if (process.platform !== "darwin") {
      app.quit();
    }
  });

  app.on("before-quit", () => {
    isQuitting = true;
    terminateServerProcess();
    if (castSender) {
      castSender.destroy();
      castSender = null;
    }
  });

  process.on("SIGINT", () => {
    terminateServerProcess();
    process.exit(0);
  });

  process.on("SIGTERM", () => {
    terminateServerProcess();
    process.exit(0);
  });

  process.on("exit", () => {
    terminateServerProcess();
  });
}
