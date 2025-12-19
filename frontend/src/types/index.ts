export type SiteStatus = 'pending' | 'active' | 'down' | 'dead' | 'frozen' | 'moved'
export type ScannerType = 'http' | 'spa'
export type CaptchaType = 'none' | 'recaptcha' | 'hcaptcha' | 'cloudflare' | 'ddos-guard' | 'dle-antibot' | 'ucoz' | 'pirate' | 'custom'

export interface ActiveTaskProgress {
  total: number
  success: number
  failed: number
}

export interface LastScanResult {
  success: number
  total: number
  status: TaskStatus
}

export interface Site {
  id: string
  domain: string
  status: SiteStatus
  cms?: string
  has_sitemap: boolean
  sitemap_urls?: string[]
  last_scan_at?: string
  next_scan_at?: string
  failure_count: number
  scan_interval_h: number
  scanner_type?: ScannerType
  captcha_type?: CaptchaType
  freeze_reason?: string
  moved_to_domain?: string
  moved_at?: string
  original_domain?: string
  created_at: string
  violations_count: number
  active_stage?: TaskStage
  active_task_progress?: ActiveTaskProgress
  pending_urls_count?: number
  last_scan?: LastScanResult
}

export interface CreateSiteRequest {
  domain: string
  cms?: string
  has_sitemap?: boolean
  sitemap_urls?: string[]
  scan_interval_h?: number
}

export interface ScanSitesRequest {
  site_ids: string[]
}

export interface ScanSitesResponse {
  message: string
  site_count: number
  task_ids: string[]
}

export interface ScanStageResponse {
  task_id: string
  message: string
}

export interface ExternalIds {
  kpid?: string
  imdb_id?: string
  tmdb_id?: string
}

export interface Page {
  id: string
  site_id: string
  url: string
  title: string
  year?: number
  external_ids: ExternalIds
  player_url?: string
  http_status: number
  indexed_at: string
}

export type TaskStatus = 'pending' | 'processing' | 'completed' | 'failed' | 'cancelled'
export type TaskStage = 'sitemap' | 'page' | 'done'

export interface StageResult {
  status: TaskStatus
  total: number
  success: number
  failed: number
  error?: string
  started_at?: string
  finished_at?: string
}

export interface ScanTask {
  id: string
  site_id: string
  domain: string
  status: TaskStatus
  stage: TaskStage
  sitemap_result?: StageResult
  page_result?: StageResult
  created_at: string
  finished_at?: string
}

export interface PageStats {
  total: number
  with_kpid: number
  with_imdb_id: number
  with_player: number
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
}

export type ScannedSinceFilter = 'today' | 'week' | 'month'
export type HasViolationsFilter = 'true' | 'false'

export interface SitesQueryParams {
  status?: SiteStatus
  scanned_since?: ScannedSinceFilter
  has_violations?: HasViolationsFilter
  limit?: number
  offset?: number
}

export interface PagesQueryParams {
  site_id?: string
  kpid?: string
  imdb_id?: string
  title?: string
  year?: number
  has_player?: boolean
  has_violations?: boolean
  sort_by?: 'indexed_at' | 'year'
  sort_order?: 'asc' | 'desc'
  limit?: number
  offset?: number
}

export interface TasksQueryParams {
  site_id?: string
  domain?: string
  status?: TaskStatus
  limit?: number
  offset?: number
}

export interface Content {
  id: string
  title: string
  original_title?: string
  year?: number
  kinopoisk_id?: string
  imdb_id?: string
  mal_id?: string
  shikimori_id?: string
  mydramalist_id?: string
  created_at: string
}

export interface ContentWithStats extends Content {
  violations_count: number
  sites_count: number
}

export interface CreateContentRequest {
  title: string
  original_title?: string
  year?: number
  kinopoisk_id?: string
  imdb_id?: string
  mal_id?: string
  shikimori_id?: string
  mydramalist_id?: string
}

export type ContentSortBy = 'violations_count' | 'created_at'
export type ContentHasViolations = 'true' | 'false'

export interface ContentQueryParams {
  title?: string
  kinopoisk_id?: string
  imdb_id?: string
  mal_id?: string
  shikimori_id?: string
  mydramalist_id?: string
  has_violations?: ContentHasViolations
  sort_by?: ContentSortBy
  sort_order?: 'asc' | 'desc'
  limit?: number
  offset?: number
}

export interface Violation {
  page_id: string
  site_id: string
  domain: string
  url: string
  title: string
  match_type: string
  found_at: string
}

export interface ViolationsQueryParams {
  limit?: number
  offset?: number
}

export type SitemapURLStatus = 'pending' | 'indexed' | 'error' | 'skipped'

export interface SitemapURL {
  id: string
  site_id: string
  url: string
  sitemap_source: string
  lastmod?: string
  priority?: number
  changefreq?: string
  status: SitemapURLStatus
  discovered_at: string
  indexed_at?: string
  error?: string
  is_xml: boolean
}

export interface SitemapURLsResponse {
  urls: SitemapURL[]
  total: number
  limit: number
  page: number
}

export interface SitemapURLStats {
  pending: number
  indexed: number
  error: number
  skipped: number
  total: number
}

export interface User {
  id: string
  login: string
  role: 'admin' | 'user'
  is_active: boolean
  created_at: string
}

export interface TokenResponse {
  access_token: string
  refresh_token: string
  expires_in: number
}

export interface UsersListResponse {
  items: User[]
  total: number
}

export interface CreateUserRequest {
  login: string
  password: string
  role: 'admin' | 'user'
}

export interface UpdateUserRequest {
  login?: string
  password?: string
  is_active?: boolean
}
