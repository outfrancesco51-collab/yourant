/**
 * Yourant Desktop - CastSender & Renderer IPC Helper
 *
 * Implements Chromecast discovery and media streaming over LAN matching Seanime architecture,
 * plus renderer IPC helper utilities for frameless window controls and server queries.
 */

const { EventEmitter } = require("events");
const os = require("os");

// Lazy/safe loading for optional dependencies so tests & headless environments run cleanly
let mdns = null;
try {
  mdns = require("mdns-js");
} catch (_) {
  // Will be handled gracefully in startDiscovery
}

let castv2 = null;
try {
  castv2 = require("castv2");
} catch (_) {
  // Will be handled gracefully in connect
}

const CAST_NAMESPACE = "urn:x-cast:app.yourant.cast";
const CAST_MEDIA_NAMESPACE = "urn:x-cast:com.google.cast.media";
const CAST_RECEIVER_NAMESPACE = "urn:x-cast:com.google.cast.receiver";
const CAST_CONNECTION_NAMESPACE = "urn:x-cast:com.google.cast.tp.connection";
const CAST_HEARTBEAT_NAMESPACE = "urn:x-cast:com.google.cast.tp.heartbeat";

// Default Chromecast Media Receiver Application ID
const CAST_APP_ID = "CC1AD845";

/**
 * CastSender manages Google Cast device discovery via mDNS, CastV2 TLS socket
 * connection, LAN URL translation for local video streaming, and media session control.
 */
class CastSender extends EventEmitter {
  constructor() {
    super();
    this.devices = new Map();
    this.browser = null;
    this.client = null;
    this.connectedDevice = null;
    this.session = null;
    this.mediaSessionId = null;
    this.heartbeatInterval = null;
    this.currentMediaStatus = null;
    this._requestId = 0;
    this._appChannelsSetup = false;
    this._mediaChannel = null;
    this._customChannel = null;
    this._connectionChannel = null;
    this._heartbeatChannel = null;
    this._receiverChannel = null;
    this._appConnectionChannel = null;
    this._transportId = null;
  }

  _nextRequestId() {
    return ++this._requestId;
  }

  /**
   * Returns the machine's primary non-internal IPv4 LAN address.
   * Remote Cast devices cannot reach localhost / 127.0.0.1, so streams must be bound to this IP.
   */
  getLanIP() {
    try {
      const interfaces = os.networkInterfaces();
      for (const name of Object.keys(interfaces)) {
        for (const iface of interfaces[name] || []) {
          if (iface.family === "IPv4" && !iface.internal) {
            return iface.address;
          }
        }
      }
    } catch (_) {
      // Fallback
    }
    return "127.0.0.1";
  }

  /**
   * Rewrites localhost/127.0.0.1 URLs into the machine's LAN IP so the Chromecast
   * can stream video files and subtitle tracks across the local network.
   */
  rewriteUrlForCast(url, serverPort = 8080) {
    if (!url || typeof url !== "string") return url;
    const lanIP = this.getLanIP();
    return url
      .replace(/127\.0\.0\.1/g, lanIP)
      .replace(/localhost/g, lanIP)
      .replace(/{{SERVER_URL}}/g, `http://${lanIP}:${serverPort}`);
  }

  /**
   * Starts mDNS discovery listening for _googlecast._tcp devices on the local network.
   */
  startDiscovery() {
    this.devices.clear();

    if (!mdns) {
      console.warn("[CastSender] mdns-js not available, discovery running in fallback mode");
      return;
    }

    try {
      this.browser = mdns.createBrowser(mdns.tcp("googlecast"));

      this.browser.on("ready", () => {
        if (this.browser) {
          this.browser.discover();
        }
      });

      this.browser.on("update", (service) => {
        if (!service || !service.addresses || service.addresses.length === 0) return;

        const deviceId = service.fullname || service.host;
        const txtRecord = service.txt || [];
        let friendlyName = deviceId;

        if (Array.isArray(txtRecord)) {
          for (const record of txtRecord) {
            if (typeof record === "string" && record.startsWith("fn=")) {
              friendlyName = record.substring(3);
            }
          }
        }

        const device = {
          id: deviceId,
          name: friendlyName,
          host: service.addresses[0],
          port: service.port || 8009,
        };

        if (!this.devices.has(deviceId)) {
          this.devices.set(deviceId, device);
          this.emit("deviceFound", device);
        }
      });

      this.browser.on("error", (err) => {
        this.emit("error", { type: "discovery", message: err.message });
      });
    } catch (err) {
      this.emit("error", { type: "discovery", message: err.message });
    }
  }

  /**
   * Stops active mDNS device discovery.
   */
  stopDiscovery() {
    if (this.browser) {
      try {
        this.browser.stop();
      } catch (_) {
        // Ignore
      }
      this.browser = null;
    }
  }

  /**
   * Returns list of all discovered Cast devices.
   */
  getDevices() {
    return Array.from(this.devices.values());
  }

  /**
   * Connects to a Chromecast device over TLS and initializes the default media receiver.
   */
  async connect(deviceId) {
    const device = this.devices.get(deviceId);
    if (!device) {
      throw new Error(`Cast device not found: ${deviceId}`);
    }

    if (!castv2) {
      throw new Error("castv2 library not available on this platform");
    }

    return new Promise((resolve, reject) => {
      this.client = new castv2.Client();

      this.client.on("error", (err) => {
        this.emit("error", { type: "connection", message: err.message });
        this._cleanup();
        reject(err);
      });

      this.client.connect({ host: device.host, port: device.port }, () => {
        this.connectedDevice = device;
        this._setupPlatformChannels();

        this._launchApp()
          .then((session) => {
            this.session = session;
            this.emit("sessionUpdate", {
              connected: true,
              device: device,
              sessionId: session.sessionId,
            });
            resolve(session);
          })
          .catch(reject);
      });
    });
  }

  _setupPlatformChannels() {
    this._connectionChannel = this.client.createChannel(
      "sender-0",
      "receiver-0",
      CAST_CONNECTION_NAMESPACE,
      "JSON"
    );
    this._connectionChannel.send({ type: "CONNECT" });

    this._heartbeatChannel = this.client.createChannel(
      "sender-0",
      "receiver-0",
      CAST_HEARTBEAT_NAMESPACE,
      "JSON"
    );

    this._heartbeatChannel.on("message", (data) => {
      if (data && data.type === "PING") {
        this._heartbeatChannel.send({ type: "PONG" });
      }
    });

    // Send heartbeat PING every 5 seconds
    this.heartbeatInterval = setInterval(() => {
      try {
        if (this._heartbeatChannel) {
          this._heartbeatChannel.send({ type: "PING" });
        }
      } catch (_) {
        this._cleanup();
      }
    }, 5000);

    this._receiverChannel = this.client.createChannel(
      "sender-0",
      "receiver-0",
      CAST_RECEIVER_NAMESPACE,
      "JSON"
    );

    this._receiverChannel.on("message", (data) => {
      if (data && data.type === "RECEIVER_STATUS") {
        this._handleReceiverStatus(data.status);
      }
    });
  }

  _handleReceiverStatus(status) {
    if (status && status.applications) {
      const app = status.applications.find((a) => a.appId === CAST_APP_ID);
      if (app) {
        this._transportId = app.transportId;
        this._setupAppChannels(app.transportId);
      }
    }
  }

  _setupAppChannels(transportId) {
    if (this._appChannelsSetup) return;
    this._appChannelsSetup = true;

    this._appConnectionChannel = this.client.createChannel(
      "sender-0",
      transportId,
      CAST_CONNECTION_NAMESPACE,
      "JSON"
    );
    this._appConnectionChannel.send({ type: "CONNECT" });

    this._mediaChannel = this.client.createChannel(
      "sender-0",
      transportId,
      CAST_MEDIA_NAMESPACE,
      "JSON"
    );

    this._mediaChannel.on("message", (data) => {
      this._handleMediaMessage(data);
    });

    this._customChannel = this.client.createChannel(
      "sender-0",
      transportId,
      CAST_NAMESPACE,
      "JSON"
    );

    this._customChannel.on("message", (data) => {
      this._handleCustomMessage(data);
    });
  }

  _launchApp() {
    return new Promise((resolve, reject) => {
      const requestId = this._nextRequestId();

      const handler = (data) => {
        if (data && data.requestId === requestId) {
          this._receiverChannel.removeListener("message", handler);

          if (data.type === "RECEIVER_STATUS" && data.status && data.status.applications) {
            const app = data.status.applications.find((a) => a.appId === CAST_APP_ID);
            if (app) {
              resolve({
                sessionId: app.sessionId,
                transportId: app.transportId,
              });
              return;
            }
          }
          reject(new Error("Failed to launch Chromecast receiver application"));
        }
      };

      this._receiverChannel.on("message", handler);

      this._receiverChannel.send({
        type: "LAUNCH",
        appId: CAST_APP_ID,
        requestId: requestId,
      });

      setTimeout(() => {
        if (this._receiverChannel) {
          this._receiverChannel.removeListener("message", handler);
        }
        reject(new Error("Chromecast app launch timed out"));
      }, 15000);
    });
  }

  _handleMediaMessage(data) {
    if (data && data.type === "MEDIA_STATUS" && data.status && data.status.length > 0) {
      const status = data.status[0];
      this.mediaSessionId = status.mediaSessionId;
      this.currentMediaStatus = status;

      this.emit("mediaStatus", {
        mediaSessionId: status.mediaSessionId,
        playerState: status.playerState,
        currentTime: status.currentTime,
        duration: status.media ? status.media.duration : undefined,
        volume: status.volume,
        idleReason: status.idleReason,
      });
    }
  }

  _handleCustomMessage(data) {
    if (!data) return;
    switch (data.type) {
      case "receiverReady":
        this.emit("receiverReady");
        break;
      case "subtitleTrackChanged":
        this.emit("subtitleTrackChanged", data.payload);
        break;
      case "error":
        this.emit("error", { type: "receiver", message: data.payload });
        break;
      default:
        break;
    }
  }

  /**
   * Loads media on the Chromecast receiver.
   * Rewrites URL to machine LAN IP, and spoofs MKV/WebM to video/mp4 for HTML5 receiver playback.
   */
  loadMedia({ streamUrl, contentType, title, subtitle, imageUrl, duration, serverPort = 8080 }) {
    if (!this._mediaChannel) {
      throw new Error("Not connected to a Cast device");
    }

    const castUrl = this.rewriteUrlForCast(streamUrl, serverPort);

    // Spoof MKV or WebM as MP4 for browser-based Chromecast receiver compatibility
    let castContentType = contentType || "video/mp4";
    if (castContentType === "video/x-matroska" || castContentType === "video/webm") {
      castContentType = "video/mp4";
    }

    const requestId = this._nextRequestId();

    const loadRequest = {
      type: "LOAD",
      requestId: requestId,
      media: {
        contentId: castUrl,
        contentType: castContentType,
        streamType: "BUFFERED",
        metadata: {
          type: 0,
          metadataType: 0,
          title: title || "Yourant Media",
          subtitle: subtitle || "",
          images: imageUrl ? [{ url: imageUrl }] : [],
        },
        ...(duration ? { duration } : {}),
      },
      autoplay: true,
      currentTime: 0,
    };

    this._mediaChannel.send(loadRequest);
    return requestId;
  }

  play() {
    if (!this._mediaChannel || !this.mediaSessionId) return;
    this._mediaChannel.send({
      type: "PLAY",
      mediaSessionId: this.mediaSessionId,
      requestId: this._nextRequestId(),
    });
  }

  pause() {
    if (!this._mediaChannel || !this.mediaSessionId) return;
    this._mediaChannel.send({
      type: "PAUSE",
      mediaSessionId: this.mediaSessionId,
      requestId: this._nextRequestId(),
    });
  }

  seek(time) {
    if (!this._mediaChannel || !this.mediaSessionId) return;
    this._mediaChannel.send({
      type: "SEEK",
      mediaSessionId: this.mediaSessionId,
      currentTime: time,
      requestId: this._nextRequestId(),
    });
  }

  stop() {
    if (!this._mediaChannel || !this.mediaSessionId) return;
    this._mediaChannel.send({
      type: "STOP",
      mediaSessionId: this.mediaSessionId,
      requestId: this._nextRequestId(),
    });
  }

  setVolume(level) {
    if (!this._receiverChannel) return;
    this._receiverChannel.send({
      type: "SET_VOLUME",
      volume: { level: Math.max(0, Math.min(1, level)) },
      requestId: this._nextRequestId(),
    });
  }

  setMuted(muted) {
    if (!this._receiverChannel) return;
    this._receiverChannel.send({
      type: "SET_VOLUME",
      volume: { muted: Boolean(muted) },
      requestId: this._nextRequestId(),
    });
  }

  getMediaStatus() {
    if (!this._mediaChannel) return;
    this._mediaChannel.send({
      type: "GET_STATUS",
      requestId: this._nextRequestId(),
    });
  }

  // Subtitle & Font relay
  sendSubtitleEvents(events) {
    if (!this._customChannel) return;
    this._customChannel.send({
      type: "subtitleEvents",
      payload: events,
    });
  }

  sendSubtitleTracks(tracks) {
    if (!this._customChannel) return;
    this._customChannel.send({
      type: "setTracks",
      payload: tracks,
    });
  }

  switchSubtitleTrack(trackNumber) {
    if (!this._customChannel) return;
    this._customChannel.send({
      type: "switchTrack",
      payload: { trackNumber },
    });
  }

  sendFonts(fontUrls, serverPort = 8080) {
    if (!this._customChannel) return;
    const rewritten = (fontUrls || []).map((url) => this.rewriteUrlForCast(url, serverPort));
    this._customChannel.send({
      type: "fonts",
      payload: rewritten,
    });
  }

  sendSubtitleHeader(header) {
    if (!this._customChannel) return;
    this._customChannel.send({
      type: "subtitleHeader",
      payload: header,
    });
  }

  disableSubtitles() {
    if (!this._customChannel) return;
    this._customChannel.send({
      type: "disableSubtitles",
    });
  }

  disconnect() {
    try {
      if (this._receiverChannel) {
        this._receiverChannel.send({
          type: "STOP",
          requestId: this._nextRequestId(),
        });
      }
    } catch (_) {
      // Ignore
    }

    this._cleanup();

    this.emit("sessionUpdate", {
      connected: false,
      device: null,
      sessionId: null,
    });
  }

  _cleanup() {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }

    if (this.client) {
      try {
        this.client.close();
      } catch (_) {
        // Ignore
      }
      this.client = null;
    }

    this.connectedDevice = null;
    this.session = null;
    this.mediaSessionId = null;
    this.currentMediaStatus = null;
    this._appChannelsSetup = false;
    this._mediaChannel = null;
    this._customChannel = null;
    this._connectionChannel = null;
    this._heartbeatChannel = null;
    this._receiverChannel = null;
    this._appConnectionChannel = null;
    this._transportId = null;
  }

  getStatus() {
    return {
      connected: Boolean(this.connectedDevice),
      device: this.connectedDevice,
      sessionId: this.session ? this.session.sessionId : null,
      mediaStatus: this.currentMediaStatus,
    };
  }

  destroy() {
    this.stopDiscovery();
    this.disconnect();
    this.removeAllListeners();
  }
}

/**
 * DesktopSender / Renderer IPC Helper
 * Provides a typed, promise-based bridge for renderer UI components
 * interacting with window controls, Go sidecar server, and casting.
 */
class DesktopSender {
  constructor(electronBridge) {
    this._bridge =
      electronBridge ||
      (typeof window !== "undefined" && window.electron ? window.electron : null);
  }

  isDesktop() {
    if (this._bridge && this._bridge.window) {
      return true;
    }
    return (
      typeof window !== "undefined" &&
      Boolean(window.__isElectronDesktop__)
    );
  }

  getPlatform() {
    return this._bridge ? this._bridge.platform : "web";
  }

  // Window Controls
  minimize() {
    if (this._bridge && this._bridge.window) {
      this._bridge.window.minimize();
    }
  }

  maximize() {
    if (this._bridge && this._bridge.window) {
      this._bridge.window.maximize();
    }
  }

  close() {
    if (this._bridge && this._bridge.window) {
      this._bridge.window.close();
    }
  }

  async isMaximized() {
    if (this._bridge && this._bridge.window) {
      return await this._bridge.window.isMaximized();
    }
    return false;
  }

  // Local Server Status
  async getLocalServerPort() {
    if (this._bridge && this._bridge.localServer) {
      return await this._bridge.localServer.getPort();
    }
    return 8080;
  }

  async isLocalServerReachable() {
    if (this._bridge && this._bridge.localServer) {
      return await this._bridge.localServer.isReachable();
    }
    return false;
  }

  // Cast Controls
  async discoverCastDevices() {
    if (this._bridge && this._bridge.cast) {
      return await this._bridge.cast.discover();
    }
  }

  async getCastDevices() {
    if (this._bridge && this._bridge.cast) {
      return await this._bridge.cast.getDevices();
    }
    return [];
  }

  async connectCast(deviceId) {
    if (this._bridge && this._bridge.cast) {
      return await this._bridge.cast.connect(deviceId);
    }
  }

  async disconnectCast() {
    if (this._bridge && this._bridge.cast) {
      return await this._bridge.cast.disconnect();
    }
  }

  async loadCastMedia(opts) {
    if (this._bridge && this._bridge.cast) {
      return await this._bridge.cast.loadMedia(opts);
    }
  }

  playCast() {
    if (this._bridge && this._bridge.cast) {
      this._bridge.cast.play();
    }
  }

  pauseCast() {
    if (this._bridge && this._bridge.cast) {
      this._bridge.cast.pause();
    }
  }

  seekCast(time) {
    if (this._bridge && this._bridge.cast) {
      this._bridge.cast.seek(time);
    }
  }

  stopCast() {
    if (this._bridge && this._bridge.cast) {
      this._bridge.cast.stop();
    }
  }

  setCastVolume(level) {
    if (this._bridge && this._bridge.cast) {
      this._bridge.cast.setVolume(level);
    }
  }

  on(channel, callback) {
    if (this._bridge && typeof this._bridge.on === "function") {
      return this._bridge.on(channel, callback);
    }
    return () => {};
  }
}

function createDesktopSender(bridge) {
  return new DesktopSender(bridge);
}

module.exports = {
  CastSender,
  DesktopSender,
  createDesktopSender,
  CAST_NAMESPACE,
  CAST_MEDIA_NAMESPACE,
  CAST_RECEIVER_NAMESPACE,
  CAST_CONNECTION_NAMESPACE,
  CAST_HEARTBEAT_NAMESPACE,
  CAST_APP_ID,
};
