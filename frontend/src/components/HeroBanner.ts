import { AnimeMedia } from '../api/client';

export class HeroBanner {
  private container: HTMLElement;
  private currentAnime: AnimeMedia | null = null;
  private onWatchClick?: (anime: AnimeMedia) => void;
  private onDetailClick?: (anime: AnimeMedia) => void;

  constructor(
    container: HTMLElement,
    callbacks?: {
      onWatch?: (anime: AnimeMedia) => void;
      onDetail?: (anime: AnimeMedia) => void;
    }
  ) {
    this.container = container;
    this.onWatchClick = callbacks?.onWatch;
    this.onDetailClick = callbacks?.onDetail;
  }

  public setAnime(anime: AnimeMedia) {
    this.currentAnime = anime;
    this.render();
  }

  private render() {
    if (!this.currentAnime) {
      this.container.innerHTML = '';
      this.container.style.display = 'none';
      return;
    }

    this.container.style.display = 'flex';
    const anime = this.currentAnime;
    const bgUrl = anime.bannerImage || anime.coverImage.extraLarge || anime.coverImage.large;
    const title = anime.title.english || anime.title.romaji || 'Featured Anime';
    const score = anime.averageScore ? `${anime.averageScore}%` : '88%';
    const format = anime.format || 'TV';
    const genres = (anime.genres || []).slice(0, 3);
    const desc = anime.description ? anime.description.replace(/<[^>]*>?/gm, '') : 'Experience thrilling storylines, world-class animation, and synchronized watch parties.';

    this.container.style.backgroundImage = `url('${bgUrl}')`;
    this.container.innerHTML = `
      <div class="hero-content">
        <div class="hero-badge-row">
          <span class="hero-tag">${format}</span>
          <span class="hero-tag score-badge">★ ${score}</span>
          ${anime.seasonYear ? `<span class="hero-tag">${anime.season || ''} ${anime.seasonYear}</span>` : ''}
          ${genres.map((g) => `<span class="hero-tag">${g}</span>`).join('')}
        </div>
        <h1 class="hero-title">${title}</h1>
        <p class="hero-desc">${desc}</p>
        <div class="hero-actions">
          <button id="hero-watch-btn" class="btn-blood">
            <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"></polygon></svg>
            Watch Now
          </button>
          <button id="hero-detail-btn" class="btn-metallic">
            <svg viewBox="0 0 24 24" width="18" height="18" stroke="currentColor" stroke-width="2" fill="none"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>
            Details &amp; Episodes
          </button>
        </div>
      </div>
    `;

    this.container.querySelector('#hero-watch-btn')?.addEventListener('click', () => {
      if (this.currentAnime) this.onWatchClick?.(this.currentAnime);
    });

    this.container.querySelector('#hero-detail-btn')?.addEventListener('click', () => {
      if (this.currentAnime) this.onDetailClick?.(this.currentAnime);
    });
  }
}
