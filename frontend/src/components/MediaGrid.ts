import { AnimeMedia, UserLibraryEntry } from '../api/client';

export class MediaGrid {
  private container: HTMLElement;
  private items: AnimeMedia[] = [];
  private trackingMap: Map<number, UserLibraryEntry> = new Map();
  private onCardClickCallback?: (anime: AnimeMedia) => void;

  constructor(container: HTMLElement, onCardClick?: (anime: AnimeMedia) => void) {
    this.container = container;
    this.onCardClickCallback = onCardClick;
  }

  public setData(items: AnimeMedia[], trackingList: UserLibraryEntry[] = []) {
    this.items = items;
    this.trackingMap.clear();
    for (const entry of trackingList) {
      this.trackingMap.set(entry.mediaId, entry);
    }
    this.render();
  }

  public updateTrackingEntry(entry: UserLibraryEntry) {
    this.trackingMap.set(entry.mediaId, entry);
    this.render();
  }

  private render() {
    if (this.items.length === 0) {
      this.container.innerHTML = `
        <div style="grid-column: 1 / -1; text-align: center; padding: 60px 20px; color: var(--text-muted)">
          <svg viewBox="0 0 24 24" width="48" height="48" stroke="currentColor" stroke-width="1.5" fill="none" style="margin-bottom: 12px; color: var(--metallic-cyan)"><circle cx="12" cy="12" r="10"></circle><line x1="8" y1="12" x2="16" y2="12"></line></svg>
          <div style="font-size: 1.1rem; font-weight: 600; color: var(--text-secondary)">No anime entries found</div>
          <div style="font-size: 0.85rem">Try adjusting your search terms or filters</div>
        </div>
      `;
      return;
    }

    this.container.innerHTML = this.items
      .map((anime) => {
        const title = anime.title.english || anime.title.romaji || 'Anime';
        const posterUrl = anime.coverImage.large || anime.coverImage.extraLarge;
        const score = anime.averageScore ? `★ ${Math.round(anime.averageScore / 10 * 10) / 10}` : '★ 8.5';
        const format = anime.format || 'TV';
        const tracking = this.trackingMap.get(anime.id);

        let progressHtml = '';
        if (tracking && tracking.totalEpisodes > 0) {
          const pct = Math.min(100, Math.round((tracking.progress / tracking.totalEpisodes) * 100));
          progressHtml = `
            <div class="card-progress-bar" title="${tracking.progress} / ${tracking.totalEpisodes} episodes (${pct}%)">
              <div class="card-progress-fill" style="width: ${pct}%"></div>
            </div>
          `;
        }

        const epSubtitle = tracking
          ? `EP ${tracking.progress} / ${tracking.totalEpisodes || '?'}`
          : anime.episodes
          ? `${anime.episodes} Episodes`
          : format;

        return `
          <div class="media-card" data-id="${anime.id}" tabindex="0">
            <div class="card-poster-wrapper">
              <img class="card-poster" src="${posterUrl}" alt="${title}" loading="lazy" />
              <div class="card-badges">
                <span class="badge-format">${format}</span>
                <span class="badge-score">${score}</span>
              </div>
              ${progressHtml}
            </div>
            <div class="card-info">
              <div class="card-title" title="${title}">${title}</div>
              <div class="card-subtitle">${epSubtitle}</div>
            </div>
          </div>
        `;
      })
      .join('');

    this.container.querySelectorAll('.media-card').forEach((card) => {
      card.addEventListener('click', () => {
        const id = parseInt((card as HTMLElement).dataset.id || '0', 10);
        const item = this.items.find((a) => a.id === id);
        if (item) this.onCardClickCallback?.(item);
      });
    });
  }
}
