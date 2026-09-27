// ==========================================================================
// YOURANT - REST API Client (client.ts)
// Interfaces strictly matching Go backend models (/internal/api/, /internal/storage/)
// ==========================================================================

export interface AnimeTitle {
  romaji: string;
  english?: string;
  native?: string;
}

export interface AnimeCoverImage {
  extraLarge: string;
  large: string;
  medium?: string;
  color?: string;
}

export interface AnimeDate {
  year?: number;
  month?: number;
  day?: number;
}

export interface AnimeMedia {
  id: number;
  idMal?: number;
  title: AnimeTitle;
  format: string; // TV, MOVIE, OVA, SPECIAL, ONA
  status: string; // FINISHED, RELEASING, NOT_YET_RELEASED
  description?: string;
  season?: string;
  seasonYear?: number;
  episodes?: number;
  duration?: number;
  coverImage: AnimeCoverImage;
  bannerImage?: string;
  genres: string[];
  averageScore: number;
  popularity: number;
  trending: number;
}

export interface PageInfo {
  total: number;
  perPage: number;
  currentPage: number;
  lastPage: number;
  hasNextPage: boolean;
}

export interface PageResult {
  pageInfo: PageInfo;
  items: AnimeMedia[];
}

export interface UserLibraryEntry {
  id: number;
  mediaId: number;
  title: string;
  status: 'CURRENT' | 'PLANNING' | 'COMPLETED' | 'DROPPED' | 'PAUSED' | string;
  progress: number;
  totalEpisodes: number;
  score: number;
  notes?: string;
  coverImage?: string;
  createdAt: string;
  updatedAt: string;
}

export interface TrackingUpdateRequest {
  mediaId: number;
  status?: string;
  progress?: number;
  score?: number;
  notes?: string;
  title?: string;
  totalEpisodes?: number;
  coverImage?: string;
}

export interface DownloadTask {
  id: string;
  animeId?: number;
  episodeNumber?: number;
  title: string;
  url: string;
  fileName: string;
  totalBytes: number;
  downloadedBytes: number;
  progress: number;
  status: 'pending' | 'downloading' | 'completed' | 'failed' | 'cancelled';
  createdAt: string;
  completedAt?: string;
  error?: string;
}

export interface FileInfo {
  fileName: string;
  size: number;
  modTime: string;
}

export class ApiClient {
  private baseUrl: string;

  constructor(baseUrl: string = '') {
    this.baseUrl = baseUrl;
  }

  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;
    const headers = {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    };

    const response = await fetch(url, { ...options, headers });
    if (!response.ok) {
      let errMessage = `HTTP ${response.status} ${response.statusText}`;
      try {
        const errorJson = await response.json();
        if (errorJson && errorJson.error) {
          errMessage = errorJson.error;
        }
      } catch {
        // use default error message
      }
      throw new Error(errMessage);
    }
    return response.json() as Promise<T>;
  }

  // 1. Health
  public async getHealth(): Promise<{ status: string; version: string }> {
    return this.request<{ status: string; version: string }>('/api/health');
  }

  // 2. Anime Metadata (AniList)
  public async getTrending(page = 1, perPage = 20): Promise<PageResult> {
    return this.request<PageResult>(`/api/anime/trending?page=${page}&perPage=${perPage}`);
  }

  public async getPopular(page = 1, perPage = 20): Promise<PageResult> {
    return this.request<PageResult>(`/api/anime/popular?page=${page}&perPage=${perPage}`);
  }

  public async searchAnime(query: string, page = 1, perPage = 20): Promise<PageResult> {
    const q = encodeURIComponent(query);
    return this.request<PageResult>(`/api/anime/search?q=${q}&page=${page}&perPage=${perPage}`);
  }

  public async getAnimeDetail(id: number): Promise<AnimeMedia> {
    return this.request<AnimeMedia>(`/api/anime/${id}`);
  }

  // 3. User Tracking Library
  public async getTrackingList(statusFilter = ''): Promise<UserLibraryEntry[]> {
    const q = statusFilter ? `?status=${encodeURIComponent(statusFilter)}` : '';
    const res = await this.request<{ lists: UserLibraryEntry[] }>(`/api/user/tracking/list${q}`);
    return res.lists || [];
  }

  public async updateTracking(req: TrackingUpdateRequest): Promise<UserLibraryEntry> {
    return this.request<UserLibraryEntry>('/api/user/tracking/update', {
      method: 'POST',
      body: JSON.stringify(req),
    });
  }

  // 4. Downloads & Controlled Storage
  public async getDownloads(): Promise<DownloadTask[]> {
    return this.request<DownloadTask[]>('/api/downloads');
  }

  public async startDownload(req: { url: string; animeId?: number; episodeNumber?: number; title?: string; fileName?: string }): Promise<DownloadTask> {
    return this.request<DownloadTask>('/api/downloads', {
      method: 'POST',
      body: JSON.stringify(req),
    });
  }

  public async cancelDownload(id: string): Promise<boolean> {
    await this.request<{ success: boolean; id: string }>(`/api/downloads/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
    return true;
  }

  public async listFiles(): Promise<FileInfo[]> {
    return this.request<FileInfo[]>('/api/downloads/files');
  }

  public async deleteFile(fileName: string): Promise<boolean> {
    await this.request<{ success: boolean; fileName: string }>(`/api/downloads/files/${encodeURIComponent(fileName)}`, {
      method: 'DELETE',
    });
    return true;
  }

  // 5. Video Stream URL generator
  public getStreamUrl(fileName: string): string {
    return `${this.baseUrl}/api/downloads/stream/${encodeURIComponent(fileName)}`;
  }
}

export const api = new ApiClient();
