/**
 * Yourant Desktop - Context Isolation Preload Bridge
 *
 * Securely bridges native Electron APIs to the renderer process via contextBridge.
 * Follows the principle of least privilege: Node.js internals and full ipcRenderer
 * are NOT exposed to the renderer; only explicit, allowlisted channels and methods are accessible.
 */

const { contextBridge, ipcRenderer } = require("electron");

// Strict allowlists for IPC communication
const ALLOWED_ON_CHANNELS = [
  "message",
  "crash",
  "window:minimized",
  "window:hidden",
  "window:maximized",
  "window:unmaximized",
  "window:fullscreen",
  "update-downloaded",
  "update-error",
  "update-available",
  "download-progress",
  "window:currentWindow",
  "window:isMainWindow",
  "cast:deviceFound",
  "cast:sessionUpdate",
  "cast:mediaStatus",
  "cast:receiverReady",
  "cast:error",
  "server-status",
];

const ALLOWED_SEND_CHANNELS = [
  "restart-app",
  "quit-app",
  "restart-server",
  "kill-server",
  "window:minimize",
  "window:maximize",
  "window:close",
  "window:toggleMaximize",
  "window:setFullscreen",
  "window:hide",
  "window:show",
  "startup:renderer-ready",
  "update-presence",
];

const ALLOWED_EMIT_CHANNELS = [
  "restart-server",
  "kill-server",
  "macos-activation-policy-accessory",
  "macos-activation-policy-regular",
];

// Expose protected window.electron object to the renderer
contextBridge.exposeInMainWorld("electron", {
  // Frameless Window Controls
  window: {
    minimize: () => ipcRenderer.send("window:minimize"),
    maximize: () => ipcRenderer.send("window:maximize"),
    close: () => ipcRenderer.send("window:close"),
    toggleMaximize: () => ipcRenderer.send("window:toggleMaximize"),
    isMaximized: () => ipcRenderer.invoke("window:isMaximized"),
    isMinimizable: () => ipcRenderer.invoke("window:isMinimizable"),
    isMaximizable: () => ipcRenderer.invoke("window:isMaximizable"),
    isClosable: () => ipcRenderer.invoke("window:isClosable"),
    isFullscreen: () => ipcRenderer.invoke("window:isFullscreen"),
    setFullscreen: (fullscreen) => ipcRenderer.send("window:setFullscreen", fullscreen),
    hide: () => ipcRenderer.send("window:hide"),
    show: () => ipcRenderer.send("window:show"),
  },

  // Local Go Backend Sidecar Status & Queries
  localServer: {
    getPort: () => ipcRenderer.invoke("get-local-server-port"),
    isReachable: () => ipcRenderer.invoke("is-local-server-reachable"),
  },

  // Host Platform Architecture ('win32' | 'darwin' | 'linux')
  platform: process.platform,

  // Application Lifecycle Signals
  startup: {
    ready: () => ipcRenderer.send("startup:renderer-ready"),
  },

  // Secure System Clipboard Operations
  clipboard: {
    writeText: (text) => ipcRenderer.invoke("clipboard:writeText", text),
  },

  // Safe Event Listener with strict channel allowlisting
  on: (channel, callback) => {
    if (ALLOWED_ON_CHANNELS.includes(channel) && typeof callback === "function") {
      const subscription = (_, ...args) => callback(...args);
      ipcRenderer.on(channel, subscription);

      // Return cleanup function to deregister listener
      return () => {
        ipcRenderer.removeListener(channel, subscription);
      };
    }
    console.warn(`[Preload] Blocked unauthorized IPC event listener on channel: "${channel}"`);
    return () => {};
  },

  // Safe Event Dispatcher with strict channel allowlisting
  send: (channel, ...args) => {
    if (ALLOWED_SEND_CHANNELS.includes(channel)) {
      ipcRenderer.send(channel, ...args);
    } else {
      console.warn(`[Preload] Blocked unauthorized IPC send on channel: "${channel}"`);
    }
  },

  // Seanime-compatible emit dispatcher
  emit: (channel, data) => {
    if (ALLOWED_EMIT_CHANNELS.includes(channel)) {
      ipcRenderer.send(channel, data);
    } else {
      console.warn(`[Preload] Blocked unauthorized IPC emit on channel: "${channel}"`);
    }
  },

  // Chromecast & LAN Media Streaming Controls matching Seanime
  cast: {
    discover: () => ipcRenderer.invoke("cast:discover"),
    stopDiscovery: () => ipcRenderer.invoke("cast:stopDiscovery"),
    getDevices: () => ipcRenderer.invoke("cast:getDevices"),
    connect: (deviceId) => ipcRenderer.invoke("cast:connect", deviceId),
    disconnect: () => ipcRenderer.invoke("cast:disconnect"),
    getStatus: () => ipcRenderer.invoke("cast:getStatus"),
    loadMedia: (opts) => ipcRenderer.invoke("cast:loadMedia", opts),
    play: () => ipcRenderer.invoke("cast:play"),
    pause: () => ipcRenderer.invoke("cast:pause"),
    seek: (time) => ipcRenderer.invoke("cast:seek", time),
    stop: () => ipcRenderer.invoke("cast:stop"),
    setVolume: (level) => ipcRenderer.invoke("cast:setVolume", level),
    setMuted: (muted) => ipcRenderer.invoke("cast:setMuted", muted),
    sendSubtitleEvents: (events) => ipcRenderer.invoke("cast:sendSubtitleEvents", events),
    sendSubtitleTracks: (tracks) => ipcRenderer.invoke("cast:sendSubtitleTracks", tracks),
    switchSubtitleTrack: (trackNumber) => ipcRenderer.invoke("cast:switchSubtitleTrack", trackNumber),
    sendFonts: (fontUrls, serverPort) => ipcRenderer.invoke("cast:sendFonts", fontUrls, serverPort),
    sendSubtitleHeader: (header) => ipcRenderer.invoke("cast:sendSubtitleHeader", header),
    disableSubtitles: () => ipcRenderer.invoke("cast:disableSubtitles"),
    getLanIP: () => ipcRenderer.invoke("cast:getLanIP"),
  },

  discord: {
    updatePresence: (data) => ipcRenderer.send("update-presence", data),
  },

  animeunity: {
    extractStream: (url) => ipcRenderer.invoke("extract-animeunity-stream", url),
  },
});

// Explicit environment detection flag for frontend UI components & player
contextBridge.exposeInMainWorld("__isElectronDesktop__", true);
