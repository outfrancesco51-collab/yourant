/**
 * WatchPartyModal.ts - Watch Party UI Drawer & Control Center
 * Part of Yourant Anime Tracking & Streaming Platform
 *
 * Implements:
 * - Room creation with ^WP-[A-Z0-9]{4}$ code generation & copy to clipboard
 * - Join room with real-time validation
 * - Live 3-Tier Sync Status Pill (Green / Amber / Red)
 * - Live Participant list with Host badge & ping latency
 * - Real-time chat messages display and send input
 */

import { wsClient, MemberInfo, WatchPartyMessage } from '../api/ws';
import { partySync, SyncStatus } from './WatchPartySync';
import './watchparty.css';

export interface WatchPartyModalOptions {
  onMediaSelect?: (mediaId: number, title: string) => void;
}

export class WatchPartyModal {
  private container: HTMLElement;
  private overlay: HTMLElement;
  private drawer: HTMLElement;
  private isOpenState: boolean = false;

  private currentRoomId: string = '';
  private currentUserId: string = '';
  private currentUserName: string = '';
  private isHost: boolean = false;
  private members: MemberInfo[] = [];

  // Subscribed elements
  private syncPillEl: HTMLElement | null = null;
  private syncLabelEl: HTMLElement | null = null;
  private participantsEl: HTMLElement | null = null;
  private chatMessagesEl: HTMLElement | null = null;
  private pingDisplayEl: HTMLElement | null = null;

  constructor(_options?: WatchPartyModalOptions) {
    // Generate or retrieve persistent user identity
    this.initUserIdentity();

    // Create DOM nodes
    this.container = document.createElement('div');
    this.container.id = 'watchparty-modal-root';
    document.body.appendChild(this.container);

    this.overlay = document.createElement('div');
    this.overlay.className = 'watchparty-drawer-overlay';
    this.container.appendChild(this.overlay);

    this.drawer = document.createElement('div');
    this.drawer.className = 'watchparty-drawer';
    this.container.appendChild(this.drawer);

    this.render();
    this.bindEvents();
    this.subscribeToSync();
  }

  private initUserIdentity() {
    let savedId = localStorage.getItem('yourant_wp_user_id');
    if (!savedId) {
      savedId = 'u-' + Math.random().toString(36).substring(2, 8);
      localStorage.setItem('yourant_wp_user_id', savedId);
    }
    this.currentUserId = savedId;

    let savedName = localStorage.getItem('yourant_wp_user_name');
    if (!savedName) {
      savedName = 'Francy';
      localStorage.setItem('yourant_wp_user_name', savedName);
    }
    this.currentUserName = savedName;
  }

  public open() {
    this.isOpenState = true;
    this.drawer.classList.add('open');
    this.overlay.classList.add('open');
  }

  public close() {
    this.isOpenState = false;
    this.drawer.classList.remove('open');
    this.overlay.classList.remove('open');
  }

  public toggle() {
    if (this.isOpenState) {
      this.close();
    } else {
      this.open();
    }
  }

  public isOpen(): boolean {
    return this.isOpenState;
  }

  private render() {
    const isConnected = wsClient.isConnected();

    this.drawer.innerHTML = `
      <header class="wp-header">
        <div class="wp-title-group">
          <svg class="wp-title-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path>
            <circle cx="9" cy="7" r="4"></circle>
            <path d="M23 21v-2a4 4 0 0 0-3-3.87"></path>
            <path d="M16 3.13a4 4 0 0 1 0 7.75"></path>
          </svg>
          <span class="wp-title">Watch Party</span>
        </div>
        <button id="wp-close-btn" class="wp-close-btn" aria-label="Close Drawer">
          <svg viewBox="0 0 24 24" width="20" height="20" stroke="currentColor" stroke-width="2" fill="none">
            <line x1="18" y1="6" x2="6" y2="18"></line>
            <line x1="6" y1="6" x2="18" y2="18"></line>
          </svg>
        </button>
      </header>

      <div class="wp-body">
        ${!isConnected ? this.renderLobbyHTML() : this.renderRoomHTML()}
      </div>
    `;

    this.queryElements();
  }

  private renderLobbyHTML(): string {
    return `
      <!-- User Profile Setup -->
      <div class="wp-card">
        <div class="wp-card-header">Your Identity</div>
        <div class="wp-input-group">
          <label class="wp-input-label">Display Name</label>
          <input type="text" id="wp-username-input" class="wp-input" value="${this.escapeHtml(this.currentUserName)}" placeholder="Enter your display name" />
        </div>
      </div>

      <!-- Host a Room -->
      <div class="wp-card">
        <div class="wp-card-header">Host a Watch Party</div>
        <p style="font-size: 0.85rem; color: var(--text-secondary); line-height: 1.4;">
          Start a synchronized session as Host. All guests will automatically synchronize with your playback.
        </p>
        <button id="wp-create-room-btn" class="wp-btn">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="12" y1="5" x2="12" y2="19"></line>
            <line x1="5" y1="12" x2="19" y2="12"></line>
          </svg>
          Create Room (WP-XXXX)
        </button>
      </div>

      <!-- Join a Room -->
      <div class="wp-card">
        <div class="wp-card-header">Join Existing Room</div>
        <div class="wp-input-group">
          <label class="wp-input-label">Room Code (Pattern: WP-XXXX)</label>
          <div class="wp-input-row">
            <input type="text" id="wp-join-code-input" class="wp-input" placeholder="e.g. WP-7F3A" maxlength="7" />
            <button id="wp-join-room-btn" class="wp-btn">Join</button>
          </div>
          <div id="wp-join-error" class="wp-validation-error hidden"></div>
        </div>
      </div>
    `;
  }

  private renderRoomHTML(): string {
    return `
      <!-- Active Room Status Banner -->
      <div class="wp-card">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <div class="wp-card-header">Active Room</div>
          <div id="wp-sync-pill" class="wp-sync-pill pill-green">
            <span class="wp-sync-dot"></span>
            <span id="wp-sync-label">SYNCED</span>
          </div>
        </div>

        <div class="wp-room-code-box">
          <span class="wp-room-code">${this.escapeHtml(this.currentRoomId)}</span>
          <button id="wp-copy-code-btn" class="wp-copy-btn" title="Copy Room Code">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
              <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
            </svg>
            <span id="wp-copy-label">Copy</span>
          </button>
        </div>

        <div style="display: flex; justify-content: space-between; align-items: center; font-size: 0.8rem; color: var(--text-muted);">
          <span>Role: <strong style="color: ${this.isHost ? 'var(--blood-bright)' : 'var(--metallic-cyan)'};">${this.isHost ? 'HOST' : 'GUEST'}</strong></span>
          <span id="wp-ping-display">Ping: -- ms</span>
        </div>

        <button id="wp-leave-btn" class="wp-btn wp-btn-danger" style="margin-top: 0.5rem;">
          Leave Room
        </button>
      </div>

      <!-- Member Roster -->
      <div class="wp-card">
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <div class="wp-card-header">Participants</div>
          <span style="font-size: 0.8rem; color: var(--text-muted);">${this.members.length} / 100</span>
        </div>
        <div id="wp-participant-list" class="wp-participant-list">
          ${this.renderParticipantsHTML()}
        </div>
      </div>

      <!-- Real-Time Chat Stream -->
      <div class="wp-chat-container">
        <div id="wp-chat-messages" class="wp-chat-messages">
          <div class="wp-chat-msg" style="color: var(--text-muted); font-size: 0.8rem; text-align: center;">
            Joined room ${this.escapeHtml(this.currentRoomId)}. Real-time chat connected.
          </div>
        </div>
        <div class="wp-chat-input-bar">
          <input type="text" id="wp-chat-input" class="wp-chat-input" placeholder="Type a message..." maxlength="500" />
          <button id="wp-chat-send-btn" class="wp-chat-send-btn" aria-label="Send">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
              <path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/>
            </svg>
          </button>
        </div>
      </div>
    `;
  }

  private renderParticipantsHTML(): string {
    if (this.members.length === 0) {
      return `<div style="font-size: 0.8rem; color: var(--text-muted); padding: 0.5rem 0;">No participants</div>`;
    }

    return this.members
      .map(
        (m) => `
        <div class="wp-participant-item ${m.isHost ? 'is-host' : ''}">
          <div class="wp-participant-info">
            <span class="wp-participant-name">${this.escapeHtml(m.name)}${m.id === this.currentUserId ? ' (You)' : ''}</span>
            ${m.isHost ? `<span class="wp-badge-host">Host</span>` : ''}
          </div>
          <span class="wp-participant-ping">${wsClient.getLatency()}ms</span>
        </div>
      `
      )
      .join('');
  }

  private queryElements() {
    this.syncPillEl = this.drawer.querySelector('#wp-sync-pill');
    this.syncLabelEl = this.drawer.querySelector('#wp-sync-label');
    this.participantsEl = this.drawer.querySelector('#wp-participant-list');
    this.chatMessagesEl = this.drawer.querySelector('#wp-chat-messages');
    this.pingDisplayEl = this.drawer.querySelector('#wp-ping-display');
  }

  private bindEvents() {
    // Drawer close
    this.drawer.querySelector('#wp-close-btn')?.addEventListener('click', () => this.close());
    this.overlay.addEventListener('click', () => this.close());

    // Host room creation
    this.drawer.querySelector('#wp-create-room-btn')?.addEventListener('click', async () => {
      await this.handleCreateRoom();
    });

    // Join room
    this.drawer.querySelector('#wp-join-room-btn')?.addEventListener('click', async () => {
      await this.handleJoinRoom();
    });

    // Username input save
    const nameInput = this.drawer.querySelector('#wp-username-input') as HTMLInputElement | null;
    nameInput?.addEventListener('change', () => {
      const val = nameInput.value.trim();
      if (val) {
        this.currentUserName = val;
        localStorage.setItem('yourant_wp_user_name', val);
      }
    });

    // Copy room code button
    this.drawer.querySelector('#wp-copy-code-btn')?.addEventListener('click', () => {
      if (this.currentRoomId) {
        navigator.clipboard.writeText(this.currentRoomId).then(() => {
          const label = this.drawer.querySelector('#wp-copy-label');
          if (label) {
            label.textContent = 'Copied!';
            setTimeout(() => {
              if (label) label.textContent = 'Copy';
            }, 2000);
          }
        });
      }
    });

    // Leave room button
    this.drawer.querySelector('#wp-leave-btn')?.addEventListener('click', () => {
      wsClient.disconnect();
      this.currentRoomId = '';
      this.isHost = false;
      this.members = [];
      partySync.setIsHost(false);
      this.render();
      this.bindEvents();
    });

    // Chat send button & Enter key
    const chatInput = this.drawer.querySelector('#wp-chat-input') as HTMLInputElement | null;
    const sendBtn = this.drawer.querySelector('#wp-chat-send-btn');

    const handleSendChat = () => {
      if (!chatInput) return;
      const text = chatInput.value.trim();
      if (text.length === 0) return;

      wsClient.send({
        type: 'chat:message',
        payload: {
          chatText: text,
          senderName: this.currentUserName,
        },
      });

      chatInput.value = '';
    };

    sendBtn?.addEventListener('click', handleSendChat);
    chatInput?.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        handleSendChat();
      }
    });

    // WS Event Listeners
    wsClient.onMessage('room:joined', (msg) => {
      if (msg.payload) {
        this.currentRoomId = msg.roomId;
        this.isHost = !!msg.payload.isHost;
        this.members = (msg.payload.members as MemberInfo[]) || [];
        partySync.setIsHost(this.isHost);
        this.render();
        this.bindEvents();
      }
    });

    wsClient.onMessage('room:user_joined', (msg) => {
      if (msg.payload && msg.payload.members) {
        this.members = msg.payload.members as MemberInfo[];
        if (this.participantsEl) {
          this.participantsEl.innerHTML = this.renderParticipantsHTML();
        }
      }
    });

    wsClient.onMessage('room:user_left', (msg) => {
      if (msg.payload) {
        if (msg.payload.members) {
          this.members = msg.payload.members as MemberInfo[];
        }
        if (msg.payload.newHostId) {
          this.isHost = (msg.payload.newHostId === this.currentUserId);
          partySync.setIsHost(this.isHost);
        }
        if (this.participantsEl) {
          this.participantsEl.innerHTML = this.renderParticipantsHTML();
        }
      }
    });

    wsClient.onMessage('chat:message', (msg) => {
      this.appendChatMessage(msg);
      // Broadcast custom event so Player floating chat can also pick it up!
      window.dispatchEvent(new CustomEvent('yourant:wp-chat', { detail: msg }));
    });

    wsClient.onLatencyChange((latency) => {
      if (this.pingDisplayEl) {
        this.pingDisplayEl.textContent = `Ping: ${latency} ms`;
      }
    });
  }

  private async handleCreateRoom() {
    try {
      // Generate standard room code via random generator
      const code = this.generateLocalRoomCode();
      this.currentRoomId = code;
      this.isHost = true;

      await wsClient.connect({
        roomId: code,
        userId: this.currentUserId,
        name: this.currentUserName,
        isHost: true,
      });

      partySync.setIsHost(true);
      this.render();
      this.bindEvents();
    } catch (err) {
      console.error('Failed to create room:', err);
    }
  }

  private async handleJoinRoom() {
    const input = this.drawer.querySelector('#wp-join-code-input') as HTMLInputElement | null;
    const errorEl = this.drawer.querySelector('#wp-join-error');
    if (!input) return;

    const rawCode = input.value.trim().toUpperCase();
    const pattern = /^WP-[A-Z0-9]{4}$/;

    if (!pattern.test(rawCode)) {
      if (errorEl) {
        errorEl.textContent = 'Room code must match ^WP-[A-Z0-9]{4}$ (e.g. WP-7F3A)';
        errorEl.classList.remove('hidden');
      }
      return;
    }

    if (errorEl) errorEl.classList.add('hidden');

    try {
      this.currentRoomId = rawCode;
      this.isHost = false;

      await wsClient.connect({
        roomId: rawCode,
        userId: this.currentUserId,
        name: this.currentUserName,
        isHost: false,
      });

      partySync.setIsHost(false);
      this.render();
      this.bindEvents();
    } catch (err) {
      if (errorEl) {
        errorEl.textContent = 'Failed to connect to room. Please check the code.';
        errorEl.classList.remove('hidden');
      }
    }
  }

  private generateLocalRoomCode(): string {
    const chars = '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ';
    let code = 'WP-';
    for (let i = 0; i < 4; i++) {
      code += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return code;
  }

  private subscribeToSync() {
    partySync.onSyncStatus((status: SyncStatus) => {
      this.updateSyncPillUI(status);
    });
  }

  private updateSyncPillUI(status: SyncStatus) {
    if (!this.syncPillEl || !this.syncLabelEl) return;

    this.syncPillEl.className = `wp-sync-pill pill-${status.pillColor}`;
    this.syncLabelEl.textContent = status.label;
  }

  private appendChatMessage(msg: WatchPartyMessage) {
    if (!this.chatMessagesEl) return;

    const text = msg.payload?.chatText || '';
    const author = msg.senderName || 'Anonymous';
    const isHost = !!msg.isHost;

    const msgEl = document.createElement('div');
    msgEl.className = 'wp-chat-msg';
    msgEl.innerHTML = `
      <div class="wp-chat-meta">
        <span class="wp-chat-author ${isHost ? 'is-host' : ''}">${this.escapeHtml(author)}${isHost ? ' (Host)' : ''}</span>
        <span>${new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</span>
      </div>
      <div class="wp-chat-text">${this.escapeHtml(text)}</div>
    `;

    this.chatMessagesEl.appendChild(msgEl);
    this.chatMessagesEl.scrollTop = this.chatMessagesEl.scrollHeight;
  }

  private escapeHtml(str: string): string {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }
}
