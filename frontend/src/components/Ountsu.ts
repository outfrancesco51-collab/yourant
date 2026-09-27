export class Ountsu {
  private container: HTMLElement;
  private isMuted: boolean = false;
  private isDeafened: boolean = false;
  private isVoiceChangerEnabled: boolean = false;

  private localStream: MediaStream | null = null;
  private peerConnection: RTCPeerConnection | null = null;
  private audioContext: AudioContext | null = null;
  private pitchShiftNode: AudioWorkletNode | null = null;
  private sourceNode: MediaStreamAudioSourceNode | null = null;
  private processedStreamDestination: MediaStreamAudioDestinationNode | null = null;
  
  private audioElement: HTMLAudioElement;

  constructor() {
    this.container = document.createElement('div');
    this.container.id = 'ountsu-widget';
    this.container.className = 'ountsu-widget';
    document.body.appendChild(this.container);

    this.audioElement = document.createElement('audio');
    this.audioElement.autoplay = true;
    document.body.appendChild(this.audioElement);

    this.render();
    this.setupListeners();
  }

  private render() {
    this.container.innerHTML = `
      <div class="ountsu-header">
        <span>Ountsu Voice <span class="ountsu-badge">SPERIMENTALE</span></span>
      </div>
      
      <div class="ountsu-input-group" id="ountsu-join-section">
        <input type="text" id="ountsu-room-id" class="ountsu-input" placeholder="Invite ID / Key" />
        <button id="ountsu-join-btn" class="ountsu-btn" style="flex: 0 0 auto;">Join</button>
      </div>

      <div class="ountsu-status" id="ountsu-status">Disconnected</div>

      <div class="ountsu-controls" id="ountsu-controls-section" style="display: none;">
        <button id="ountsu-mute-btn" class="ountsu-btn">Mute</button>
        <button id="ountsu-deafen-btn" class="ountsu-btn">Deafen</button>
        <button id="ountsu-voice-btn" class="ountsu-btn">Female Voice</button>
        <button id="ountsu-leave-btn" class="ountsu-btn active">Leave</button>
      </div>
    `;
  }

  private setupListeners() {
    const joinBtn = document.getElementById('ountsu-join-btn')!;
    const leaveBtn = document.getElementById('ountsu-leave-btn')!;
    const muteBtn = document.getElementById('ountsu-mute-btn')!;
    const deafenBtn = document.getElementById('ountsu-deafen-btn')!;
    const voiceBtn = document.getElementById('ountsu-voice-btn')!;

    joinBtn.addEventListener('click', async () => {
      const roomId = (document.getElementById('ountsu-room-id') as HTMLInputElement).value;
      if (!roomId) return;
      await this.joinRoom(roomId);
    });

    leaveBtn.addEventListener('click', () => {
      this.leaveRoom();
    });

    muteBtn.addEventListener('click', () => {
      this.isMuted = !this.isMuted;
      muteBtn.classList.toggle('active', this.isMuted);
      if (this.localStream) {
        this.localStream.getAudioTracks().forEach(t => t.enabled = !this.isMuted);
      }
    });

    deafenBtn.addEventListener('click', () => {
      this.isDeafened = !this.isDeafened;
      deafenBtn.classList.toggle('active', this.isDeafened);
      this.audioElement.muted = this.isDeafened;
    });

    voiceBtn.addEventListener('click', () => {
      this.isVoiceChangerEnabled = !this.isVoiceChangerEnabled;
      voiceBtn.classList.toggle('active', this.isVoiceChangerEnabled);
      this.updateAudioPipeline();
    });
  }

  private async joinRoom(roomId: string) {
    const statusEl = document.getElementById('ountsu-status')!;
    statusEl.innerText = `Connecting to ${roomId}...`;

    try {
      this.localStream = await navigator.mediaDevices.getUserMedia({ audio: true });
      
      // Initialize Web Audio API for Voice Changer
      this.audioContext = new AudioContext();
      await this.audioContext.audioWorklet.addModule('/jungle-worklet.js');
      
      this.sourceNode = this.audioContext.createMediaStreamSource(this.localStream);
      this.pitchShiftNode = new AudioWorkletNode(this.audioContext, 'jungle-pitch-shifter');
      this.processedStreamDestination = this.audioContext.createMediaStreamDestination();

      this.updateAudioPipeline();

      // Setup WebRTC Mock
      this.peerConnection = new RTCPeerConnection({
        iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
      });

      // Add the processed audio track to peer connection
      const processedTrack = this.processedStreamDestination.stream.getAudioTracks()[0];
      this.peerConnection.addTrack(processedTrack, this.processedStreamDestination.stream);

      // Mock receiving remote audio (loopback for testing if you want, but here we just listen)
      this.peerConnection.ontrack = (event) => {
        if (event.streams && event.streams[0]) {
          this.audioElement.srcObject = event.streams[0];
        }
      };

      // Create dummy offer to trigger ICE (normally handled via signaling server)
      const offer = await this.peerConnection.createOffer();
      await this.peerConnection.setLocalDescription(offer);

      statusEl.innerText = `Connected to ${roomId}`;
      document.getElementById('ountsu-join-section')!.style.display = 'none';
      document.getElementById('ountsu-controls-section')!.style.display = 'flex';

    } catch (err) {
      console.error('Failed to join Ountsu room:', err);
      statusEl.innerText = 'Microphone permission denied or WebRTC error.';
    }
  }

  private updateAudioPipeline() {
    if (!this.sourceNode || !this.pitchShiftNode || !this.processedStreamDestination) return;

    this.sourceNode.disconnect();
    this.pitchShiftNode.disconnect();

    if (this.isVoiceChangerEnabled) {
      this.sourceNode.connect(this.pitchShiftNode);
      this.pitchShiftNode.connect(this.processedStreamDestination);
    } else {
      this.sourceNode.connect(this.processedStreamDestination);
    }
  }

  private leaveRoom() {
    if (this.peerConnection) {
      this.peerConnection.close();
      this.peerConnection = null;
    }

    if (this.localStream) {
      this.localStream.getTracks().forEach(t => t.stop());
      this.localStream = null;
    }

    if (this.audioContext) {
      this.audioContext.close();
      this.audioContext = null;
      this.sourceNode = null;
      this.pitchShiftNode = null;
      this.processedStreamDestination = null;
    }

    this.audioElement.srcObject = null;

    document.getElementById('ountsu-status')!.innerText = 'Disconnected';
    document.getElementById('ountsu-join-section')!.style.display = 'flex';
    document.getElementById('ountsu-controls-section')!.style.display = 'none';
    (document.getElementById('ountsu-room-id') as HTMLInputElement).value = '';
  }
}
