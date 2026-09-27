import { AnimeMedia, UserLibraryEntry, api } from '../api/client';

export class MediaModal {
  private container: HTMLElement;
  private currentAnime: AnimeMedia | null = null;
  private currentTracking: UserLibraryEntry | null = null;
  private onTrackingUpdatedCallback?: (entry: UserLibraryEntry) => void;
  private onPlayEpisodeCallback?: (anime: AnimeMedia, episodeNum: number, fileName?: string) => void;

  constructor(
    container: HTMLElement,
    callbacks?: {
      onTrackingUpdated?: (entry: UserLibraryEntry) => void;
      onPlayEpisode?: (anime: AnimeMedia, episodeNum: number, fileName?: string) => void;
    }
  ) {
    this.container = container;
    this.onTrackingUpdatedCallback = callbacks?.onTrackingUpdated;
    this.onPlayEpisodeCallback = callbacks?.onPlayEpisode;
    this.initCloseEvents();
  }

  public async open(anime: AnimeMedia, existingTracking?: UserLibraryEntry) {
    this.currentAnime = anime;
    this.currentTracking = existingTracking || null;
    this.container.classList.add('active');
    document.body.style.overflow = 'hidden';

    // Fetch fresh details and downloaded files concurrently
    const [freshDetail, downloadedFiles] = await Promise.all([
      api.getAnimeDetail(anime.id).catch(() => anime),
      api.listFiles().catch(() => []),
    ]);

    this.currentAnime = freshDetail;
    this.render(downloadedFiles.map((f) => f.fileName));
  }

  public close() {
    this.container.classList.remove('active');
    this.container.innerHTML = '';
    document.body.style.overflow = '';
  }

  private initCloseEvents() {
    this.container.addEventListener('click', (e) => {
      if (e.target === this.container) {
        this.close();
      }
    });

    window.addEventListener('keydown', (e) => {
      if (e.key === 'Escape' && this.container.classList.contains('active')) {
        this.close();
      }
    });
  }

  private render(downloadedFiles: string[]) {
    if (!this.currentAnime) return;
    const anime = this.currentAnime;
    const title = anime.title.english || anime.title.romaji || 'Anime Details';
    const posterUrl = anime.coverImage.extraLarge || anime.coverImage.large;
    const desc = anime.description ? anime.description.replace(/<[^>]*>?/gm, '') : 'No description available.';
    const totalEps = anime.episodes || this.currentTracking?.totalEpisodes || 12;
    const progress = this.currentTracking?.progress || 0;
    const status = this.currentTracking?.status || 'PLANNING';
    const score = this.currentTracking?.score || 0;

    const episodesArr = Array.from({ length: totalEps }, (_, i) => i + 1);

    this.container.innerHTML = `
      <div class="modal-card">
        <button class="modal-close-btn" id="modal-close" aria-label="Close modal">&times;</button>
        <div class="modal-body">
          <div class="modal-left">
            <img class="modal-poster" src="${posterUrl}" alt="${title}" />

            <!-- User Tracking Form -->
            <div class="tracking-control-box">
              <div class="tracking-row">
                <label>Tracking Status</label>
                <select id="tracking-status" class="select-input">
                  <option value="CURRENT" ${status === 'CURRENT' ? 'selected' : ''}>Watching</option>
                  <option value="PLANNING" ${status === 'PLANNING' ? 'selected' : ''}>Planning</option>
                  <option value="COMPLETED" ${status === 'COMPLETED' ? 'selected' : ''}>Completed</option>
                  <option value="PAUSED" ${status === 'PAUSED' ? 'selected' : ''}>Paused</option>
                  <option value="DROPPED" ${status === 'DROPPED' ? 'selected' : ''}>Dropped</option>
                </select>
              </div>

              <div class="tracking-row">
                <label>Episode Progress (${progress}/${totalEps})</label>
                <div style="display: flex; gap: 8px; align-items: center;">
                  <button id="btn-prog-minus" class="icon-btn" style="width: 36px; height: 36px;">-</button>
                  <input type="number" id="tracking-progress" class="num-input" value="${progress}" min="0" max="${totalEps}" style="text-align: center;" />
                  <button id="btn-prog-plus" class="icon-btn" style="width: 36px; height: 36px;">+</button>
                </div>
              </div>

              <div class="tracking-row">
                <label>My Score (0.0 – 10.0)</label>
                <input type="number" id="tracking-score" class="num-input" value="${score}" min="0" max="10" step="0.5" />
              </div>

              <button id="btn-save-tracking" class="btn-blood" style="width: 100%; justify-content: center; margin-top: 4px;">
                Save Tracking
              </button>
            </div>
          </div>

          <div class="modal-right">
            <div>
              <h2 class="modal-title">${title}</h2>
              <div style="font-size: 0.85rem; color: var(--text-muted); margin-top: 4px;">${anime.title.native || ''}</div>
            </div>

            <div class="modal-genres">
              ${(anime.genres || []).map((g) => `<span class="modal-genre-tag">${g}</span>`).join('')}
              <span class="modal-genre-tag" style="border-color: #fbbf24; color: #fbbf24">★ ${anime.averageScore || 88}%</span>
              <span class="modal-genre-tag">${anime.format || 'TV'}</span>
            </div>

            <p class="modal-desc">${desc}</p>

            <div class="episodes-section">
              <h3 style="font-size: 1.1rem; font-weight: 700;">Episodes (${totalEps})</h3>
              <div class="episode-grid">
                ${episodesArr
                  .map((epNum) => {
                    const expectedName = `anime_${anime.id}_ep${epNum}.mp4`;
                    const isDownloaded = downloadedFiles.includes(expectedName);
                    return `
                      <button class="episode-btn ${isDownloaded ? 'is-downloaded' : ''}" data-ep="${epNum}" data-file="${isDownloaded ? expectedName : ''}">
                        <div style="font-weight: 700;">Episode ${epNum}</div>
                        <div style="font-size: 0.72rem; color: ${isDownloaded ? '#10b981' : 'var(--text-muted)'}; margin-top: 2px;">
                          ${isDownloaded ? '✓ Offline Ready' : 'Stream / Play'}
                        </div>
                      </button>
                    `;
                  })
                  .join('')}
              </div>
            </div>
          </div>
        </div>
      </div>
    `;

    // Event Bindings
    this.container.querySelector('#modal-close')?.addEventListener('click', () => this.close());

    const progInput = this.container.querySelector('#tracking-progress') as HTMLInputElement;
    this.container.querySelector('#btn-prog-minus')?.addEventListener('click', () => {
      const val = Math.max(0, parseInt(progInput.value || '0', 10) - 1);
      progInput.value = val.toString();
    });
    this.container.querySelector('#btn-prog-plus')?.addEventListener('click', () => {
      const val = Math.min(totalEps, parseInt(progInput.value || '0', 10) + 1);
      progInput.value = val.toString();
    });

    this.container.querySelector('#btn-save-tracking')?.addEventListener('click', async () => {
      const st = (this.container.querySelector('#tracking-status') as HTMLSelectElement).value;
      const pr = parseInt(progInput.value || '0', 10);
      const sc = parseFloat((this.container.querySelector('#tracking-score') as HTMLInputElement).value || '0');

      try {
        const updated = await api.updateTracking({
          mediaId: anime.id,
          status: st,
          progress: pr,
          score: sc,
          title: anime.title.english || anime.title.romaji,
          totalEpisodes: totalEps,
          coverImage: posterUrl,
        });
        this.currentTracking = updated;
        this.onTrackingUpdatedCallback?.(updated);
      } catch (err: unknown) {
        console.error('Failed to update tracking:', err);
      }
    });

    this.container.querySelectorAll('.episode-btn').forEach((btn) => {
      btn.addEventListener('click', () => {
        const ep = parseInt((btn as HTMLElement).dataset.ep || '1', 10);
        const fileName = (btn as HTMLElement).dataset.file || undefined;
        this.onPlayEpisodeCallback?.(anime, ep, fileName);
      });
    });
  }
}
