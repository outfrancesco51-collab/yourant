// ==========================================================================
// YOURANT - Application Shell Bootstrap & Orchestration (main.ts)
// ==========================================================================

import { api, AnimeMedia, UserLibraryEntry } from './api/client';
import { Sidebar } from './components/Sidebar';
import { HeroBanner } from './components/HeroBanner';
import { MediaGrid } from './components/MediaGrid';
import { MediaModal } from './components/MediaModal';
import { ShaderBackground } from './glsl/ShaderBackground';
import { CinemaModeManager } from './cinema/CinemaMode';
import { Player } from './components/Player';
import { WatchPartyModal } from './party/WatchPartyModal';
import { Ountsu } from './components/Ountsu';

import vertShader from './glsl/background.vert?raw';
import fragShader from './glsl/background.frag?raw';

class App {
  private sidebar!: Sidebar;
  private heroBanner!: HeroBanner;
  private mediaGrid!: MediaGrid;
  private mediaModal!: MediaModal;
  private shaderBg!: ShaderBackground;
  private cinemaManager!: CinemaModeManager;
  private watchPartyModal!: WatchPartyModal;
  private player: Player | null = null;

  private currentTab = 'trending';
  private currentStatusFilter = 'ALL';
  private allTracking: UserLibraryEntry[] = [];
  private animeCatalog: AnimeMedia[] = [];

  public async init() {
    this.initBackgroundShader();
    this.mountComponents();
    this.setupEventListeners();
    await this.checkBackendHealth();
    await this.loadInitialData();
  }

  private initBackgroundShader() {
    const canvas = document.getElementById('bg-canvas') as HTMLCanvasElement;
    if (canvas) {
      this.shaderBg = new ShaderBackground(canvas, {
        vertSource: vertShader,
        fragSource: fragShader,
        maxDpr: 1.5,
      });
      this.cinemaManager = new CinemaModeManager(this.shaderBg);
    } else {
      this.cinemaManager = new CinemaModeManager();
    }
  }

  private mountComponents() {
    // 1. Mount Sidebar
    const sidebarContainer = document.getElementById('sidebar-container') as HTMLElement;
    this.sidebar = new Sidebar(sidebarContainer, (tab) => this.switchTab(tab));

    // 2. Mount Hero Banner
    const heroContainer = document.getElementById('hero-container') as HTMLElement;
    this.heroBanner = new HeroBanner(heroContainer, {
      onWatch: (anime) => this.openPlayer(anime, 1),
      onDetail: (anime) => this.openModal(anime),
    });

    // 3. Mount Media Grid
    const gridContainer = document.getElementById('media-grid') as HTMLElement;
    this.mediaGrid = new MediaGrid(gridContainer, (anime) => this.openModal(anime));

    // 4. Mount Detail Modal
    const modalContainer = document.getElementById('modal-container') as HTMLElement;
    this.mediaModal = new MediaModal(modalContainer, {
      onTrackingUpdated: (entry) => {
        const idx = this.allTracking.findIndex((t) => t.mediaId === entry.mediaId);
        if (idx >= 0) {
          this.allTracking[idx] = entry;
        } else {
          this.allTracking.push(entry);
        }
        this.mediaGrid.updateTrackingEntry(entry);
      },
      onPlayEpisode: (anime, ep, fileName) => {
        this.openPlayer(anime, ep, fileName);
      },
    });

    // 5. Player close event
    const playerModal = document.getElementById('player-modal');
    const playerCloseBtn = document.getElementById('player-close-btn');
    playerCloseBtn?.addEventListener('click', () => {
      this.closePlayer();
    });

    playerModal?.addEventListener('click', (e) => {
      if (e.target === playerModal) {
        this.closePlayer();
      }
    });

    window.addEventListener('keydown', (e) => {
      if (e.key === 'Escape' && playerModal && !playerModal.classList.contains('hidden') && !this.cinemaManager.isActive()) {
        this.closePlayer();
      }
    });

    // 6. Mount Watch Party Drawer Modal
    this.watchPartyModal = new WatchPartyModal();

    // 7. Mount Ountsu Voice Widget
    (window as any).ountsuWidget = new Ountsu();
  }

  private setupEventListeners() {
    // Header Watch Party button
    document.getElementById('header-watchparty-btn')?.addEventListener('click', () => {
      this.watchPartyModal.open();
    });

    // Sidebar toggle button
    document.getElementById('sidebar-toggle')?.addEventListener('click', () => {
      this.sidebar.toggle();
    });

    // Search bar with 300ms debounce
    let searchDebounce: number;
    const searchInput = document.getElementById('global-search') as HTMLInputElement;
    const searchClear = document.getElementById('search-clear') as HTMLButtonElement;

    searchInput?.addEventListener('input', () => {
      const q = searchInput.value.trim();
      searchClear.classList.toggle('hidden', q.length === 0);
      window.clearTimeout(searchDebounce);
      searchDebounce = window.setTimeout(() => this.handleSearch(q), 300);
    });

    searchClear?.addEventListener('click', () => {
      searchInput.value = '';
      searchClear.classList.add('hidden');
      this.switchTab(this.currentTab);
    });

    // Content tab buttons
    document.querySelectorAll('.tab-btn').forEach((btn) => {
      btn.addEventListener('click', () => {
        const tab = (btn as HTMLElement).dataset.tab;
        if (tab) this.switchTab(tab);
      });
    });

    // Library status filter pills
    document.querySelectorAll('.filter-pill').forEach((pill) => {
      pill.addEventListener('click', () => {
        document.querySelectorAll('.filter-pill').forEach((p) => p.classList.remove('active'));
        pill.classList.add('active');
        this.currentStatusFilter = (pill as HTMLElement).dataset.status || 'ALL';
        this.renderLibraryView();
      });
    });

    // Custom play media event listener
    window.addEventListener('yourant:play-media', ((e: CustomEvent) => {
      const { anime, episodeNum, fileName } = e.detail;
      this.openPlayer(anime, episodeNum, fileName);
    }) as EventListener);
  }

  private async checkBackendHealth() {
    const pill = document.getElementById('backend-status-pill');
    try {
      const health = await api.getHealth();
      if (pill) {
        pill.className = 'status-pill status-online';
        pill.innerHTML = `<span class="status-dot"></span><span>Backend v${health.version}</span>`;
      }
    } catch {
      if (pill) {
        pill.className = 'status-pill';
        pill.innerHTML = `<span class="status-dot" style="background: #ef4444"></span><span style="color: #ef4444">Offline</span>`;
      }
    }
  }

  private async loadInitialData() {
    try {
      const [trendingRes, trackingRes] = await Promise.all([
        api.getTrending(1, 20),
        api.getTrackingList(),
      ]);

      this.animeCatalog = trendingRes.items;
      this.allTracking = trackingRes;

      if (this.animeCatalog.length > 0) {
        this.heroBanner.setAnime(this.animeCatalog[0]);
      }
      this.mediaGrid.setData(this.animeCatalog, this.allTracking);
    } catch (err) {
      console.error('Failed to load initial data:', err);
    }
  }

  public async switchTab(tab: string) {
    this.currentTab = tab;
    this.sidebar.setActiveTab(tab);

    // Update tab bar buttons active state
    document.querySelectorAll('.tab-btn').forEach((btn) => {
      if ((btn as HTMLElement).dataset.tab === tab) {
        btn.classList.add('active');
      } else {
        btn.classList.remove('active');
      }
    });

    const libraryFilters = document.getElementById('library-filter-group');
    const heroContainer = document.getElementById('hero-container');
    const sectionTitle = document.getElementById('section-title');

    if (tab === 'trending') {
      libraryFilters?.classList.add('hidden');
      if (heroContainer) heroContainer.style.display = 'flex';
      if (sectionTitle) sectionTitle.innerText = 'Trending Anime';
      const res = await api.getTrending(1, 20);
      this.animeCatalog = res.items;
      if (this.animeCatalog.length > 0) this.heroBanner.setAnime(this.animeCatalog[0]);
      this.mediaGrid.setData(this.animeCatalog, this.allTracking);
    } else if (tab === 'popular') {
      libraryFilters?.classList.add('hidden');
      if (heroContainer) heroContainer.style.display = 'flex';
      if (sectionTitle) sectionTitle.innerText = 'All-Time Popular Anime';
      const res = await api.getPopular(1, 20);
      this.animeCatalog = res.items;
      if (this.animeCatalog.length > 0) this.heroBanner.setAnime(this.animeCatalog[0]);
      this.mediaGrid.setData(this.animeCatalog, this.allTracking);
    } else if (tab === 'library') {
      libraryFilters?.classList.remove('hidden');
      if (heroContainer) heroContainer.style.display = 'none';
      if (sectionTitle) sectionTitle.innerText = 'My Anime Library';
      this.renderLibraryView();
    } else if (tab === 'downloads') {
      libraryFilters?.classList.add('hidden');
      if (heroContainer) heroContainer.style.display = 'none';
      if (sectionTitle) sectionTitle.innerText = 'Offline Downloaded Media';
      this.renderDownloadsView();
    } else if (tab === 'watchparty') {
      libraryFilters?.classList.add('hidden');
      if (heroContainer) heroContainer.style.display = 'none';
      if (sectionTitle) sectionTitle.innerText = 'Watch Party (Host or Join Room)';
      this.mediaGrid.setData([]);
      this.watchPartyModal.open();
    }
  }

  private async renderLibraryView() {
    this.allTracking = await api.getTrackingList();
    let filtered = this.allTracking;
    if (this.currentStatusFilter !== 'ALL') {
      filtered = this.allTracking.filter((t) => t.status === this.currentStatusFilter);
    }

    const libraryItems: AnimeMedia[] = filtered.map((entry) => ({
      id: entry.mediaId,
      title: { romaji: entry.title, english: entry.title },
      format: 'TV',
      status: entry.status,
      episodes: entry.totalEpisodes,
      coverImage: {
        extraLarge: entry.coverImage || 'https://via.placeholder.com/300x450/0f172a/38bdf8?text=Yourant',
        large: entry.coverImage || 'https://via.placeholder.com/300x450/0f172a/38bdf8?text=Yourant',
      },
      genres: [entry.status],
      averageScore: Math.round(entry.score * 10),
      popularity: 0,
      trending: 0,
    }));

    this.mediaGrid.setData(libraryItems, this.allTracking);
  }

  private async renderDownloadsView() {
    const files = await api.listFiles();
    const downloadMedia: AnimeMedia[] = files.map((file, idx) => ({
      id: 990000 + idx,
      title: { romaji: file.fileName, english: file.fileName },
      format: 'OFFLINE',
      status: 'DOWNLOADED',
      episodes: 1,
      coverImage: {
        extraLarge: 'https://via.placeholder.com/300x450/0f172a/e11d48?text=OFFLINE',
        large: 'https://via.placeholder.com/300x450/0f172a/e11d48?text=OFFLINE',
      },
      genres: [`${(file.size / (1024 * 1024)).toFixed(1)} MB`],
      averageScore: 100,
      popularity: 0,
      trending: 0,
    }));

    this.mediaGrid.setData(downloadMedia, []);
  }

  private async handleSearch(query: string) {
    if (!query) {
      this.switchTab(this.currentTab);
      return;
    }
    const sectionTitle = document.getElementById('section-title');
    if (sectionTitle) sectionTitle.innerText = `Search Results for "${query}"`;
    try {
      const res = await api.searchAnime(query);
      this.animeCatalog = res.items;
      this.mediaGrid.setData(this.animeCatalog, this.allTracking);
    } catch (err) {
      console.error('Search failed:', err);
    }
  }

  private openModal(anime: AnimeMedia) {
    const tracking = this.allTracking.find((t) => t.mediaId === anime.id);
    this.mediaModal.open(anime, tracking);
  }

  private openPlayer(anime: AnimeMedia, episodeNum: number, fileName?: string) {
    const playerModal = document.getElementById('player-modal');
    const playerContainer = document.getElementById('player-container');
    if (!playerModal || !playerContainer) return;

    playerModal.classList.remove('hidden');

    const streamUrl = fileName 
      ? api.getStreamUrl(fileName)
      : 'https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4';

    if (this.player) {
      this.player.destroy();
    }

    this.player = new Player({
      container: playerContainer,
      src: streamUrl,
      title: `${anime.title.english || anime.title.romaji} - Episode ${episodeNum}`,
      poster: anime.bannerImage || anime.coverImage.extraLarge,
      cinemaManager: this.cinemaManager,
    });
  }

  private closePlayer() {
    if (this.cinemaManager.isActive()) {
      this.cinemaManager.disable();
    }
    const playerModal = document.getElementById('player-modal');
    playerModal?.classList.add('hidden');
    if (this.player) {
      this.player.destroy();
      this.player = null;
    }
  }
}

// Bootstrap when DOM is ready
window.addEventListener('DOMContentLoaded', () => {
  const app = new App();
  app.init().catch(console.error);
});
