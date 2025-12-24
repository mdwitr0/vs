export interface User {
  id: string
  login: string
  role: 'admin' | 'user'
  is_active: boolean
  created_at: string
  updated_at?: string
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

export type SiteStatus = 'active' | 'scanning' | 'error'

export interface Site {
  id: string
  user_id: string
  domain: string
  total_pages: number
  pages_with_player: number
  pages_without_player: number
  last_scan_at?: string
  status: SiteStatus
  created_at: string
}

export type PageType = 'content' | 'catalog' | 'static' | 'error'

export interface Page {
  id: string
  user_id: string
  site_id: string
  url: string
  has_player: boolean
  page_type: PageType
  exclude_from_report: boolean
  last_checked_at: string
}

export interface Settings {
  id: string
  user_id: string
  player_pattern: string
  scan_interval_hours: number
  updated_at: string
}

export interface AuditLog {
  id: string
  user_id: string
  user_login?: string
  action: string
  details?: Record<string, unknown>
  ip_address: string
  created_at: string
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
}

export interface SitesQueryParams {
  domain?: string
  status?: SiteStatus
  limit?: number
  offset?: number
}

export interface PagesQueryParams {
  site_id?: string
  has_player?: boolean
  page_type?: PageType
  exclude_from_report?: boolean
  limit?: number
  offset?: number
}

export interface AuditLogsQueryParams {
  action?: string
  date_from?: string
  date_to?: string
  limit?: number
  offset?: number
}

export interface UpdateSettingsRequest {
  player_pattern?: string
  scan_interval_hours?: number
}
