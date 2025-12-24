import type {
  Site,
  Page,
  ScanTask,
  PageStats,
  PaginatedResponse,
  SitesQueryParams,
  PagesQueryParams,
  TasksQueryParams,
  CreateSiteRequest,
  ScanSitesRequest,
  ScanSitesResponse,
  ScanStageResponse,
  ContentWithStats,
  ContentQueryParams,
  CreateContentRequest,
  Violation,
  ViolationsQueryParams,
  SitemapURLsResponse,
  SitemapURLStats,
  SitemapURLStatus,
  User,
  TokenResponse,
  UsersListResponse,
  CreateUserRequest,
  UpdateUserRequest,
} from '@/types'

const API_BASE = '/api'

let isRefreshing = false
let refreshPromise: Promise<boolean> | null = null

class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function tryRefresh(): Promise<boolean> {
  if (isRefreshing) {
    return refreshPromise!
  }

  isRefreshing = true
  refreshPromise = (async () => {
    const refreshToken = localStorage.getItem('refresh_token')
    if (!refreshToken) return false

    try {
      const res = await fetch(`${API_BASE}/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: refreshToken }),
      })
      if (!res.ok) return false

      const data = await res.json()
      localStorage.setItem('access_token', data.access_token)
      localStorage.setItem('refresh_token', data.refresh_token)
      return true
    } catch {
      return false
    } finally {
      isRefreshing = false
      refreshPromise = null
    }
  })()

  return refreshPromise
}

async function request<T>(
  endpoint: string,
  options?: RequestInit
): Promise<T> {
  const token = localStorage.getItem('access_token')
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...options?.headers,
  }
  if (token) {
    ;(headers as Record<string, string>)['Authorization'] = `Bearer ${token}`
  }

  const response = await fetch(`${API_BASE}${endpoint}`, {
    ...options,
    headers,
  })

  if (response.status === 401 && !endpoint.includes('/auth/')) {
    const refreshed = await tryRefresh()
    if (refreshed) {
      return request(endpoint, options)
    }
    window.location.href = '/login'
    throw new ApiError(401, 'Unauthorized')
  }

  if (!response.ok) {
    const message = await response.text()
    throw new ApiError(response.status, message || response.statusText)
  }

  const contentType = response.headers.get('content-type')
  if (contentType && contentType.includes('application/json')) {
    return response.json()
  }
  return undefined as T
}

function buildQueryString(params: Record<string, string | number | boolean | undefined>): string {
  const searchParams = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '' && value !== false) {
      searchParams.append(key, String(value))
    }
  })
  const queryString = searchParams.toString()
  return queryString ? `?${queryString}` : ''
}

export async function downloadFile(url: string, defaultFilename: string = 'download'): Promise<void> {
  const token = localStorage.getItem('access_token')
  const headers: HeadersInit = {}
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const response = await fetch(url, { headers })

  if (response.status === 401) {
    const refreshed = await tryRefresh()
    if (refreshed) {
      return downloadFile(url, defaultFilename)
    }
    window.location.href = '/login'
    throw new ApiError(401, 'Unauthorized')
  }

  if (!response.ok) {
    throw new ApiError(response.status, 'Download failed')
  }

  const blob = await response.blob()

  let filename = defaultFilename
  const contentDisposition = response.headers.get('content-disposition')
  if (contentDisposition) {
    const match = contentDisposition.match(/filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/)
    if (match && match[1]) {
      filename = match[1].replace(/['"]/g, '')
    }
  }

  const blobUrl = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = blobUrl
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(blobUrl)
}

export const authApi = {
  login: (login: string, password: string) =>
    request<TokenResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ login, password }),
    }),
  me: () => request<User>('/auth/me'),
  logout: () =>
    request<void>('/auth/logout', { method: 'POST' }),
}

export const usersApi = {
  list: () => request<UsersListResponse>('/users'),
  create: (data: CreateUserRequest) =>
    request<User>('/users', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: UpdateUserRequest) =>
    request<User>(`/users/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) =>
    request<void>(`/users/${id}`, { method: 'DELETE' }),
}

export const sitesApi = {
  list: (params: SitesQueryParams = {}): Promise<PaginatedResponse<Site>> => {
    const query = buildQueryString({
      status: params.status,
      scanned_since: params.scanned_since,
      has_violations: params.has_violations,
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    })
    return request<PaginatedResponse<Site>>(`/sites${query}`)
  },

  get: (id: string): Promise<Site> => {
    return request<Site>(`/sites/${id}`)
  },

  create: (data: CreateSiteRequest): Promise<Site> => {
    return request<Site>('/sites', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  createBatch: (sites: CreateSiteRequest[]): Promise<{ created: number; failed: number; site_ids: string[] }> => {
    return request<{ created: number; failed: number; site_ids: string[] }>('/sites/batch', {
      method: 'POST',
      body: JSON.stringify({ sites }),
    })
  },

  scan: (data: ScanSitesRequest): Promise<ScanSitesResponse> => {
    return request<ScanSitesResponse>('/sites/scan', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  delete: (id: string): Promise<{ message: string; pages_deleted: number; tasks_deleted: number }> => {
    return request<{ message: string; pages_deleted: number; tasks_deleted: number }>(`/sites/${id}`, {
      method: 'DELETE',
    })
  },

  deleteBulk: (siteIds: string[]): Promise<{ deleted_count: number; pages_deleted: number; tasks_deleted: number }> => {
    return request<{ deleted_count: number; pages_deleted: number; tasks_deleted: number }>('/sites/delete', {
      method: 'POST',
      body: JSON.stringify({ site_ids: siteIds }),
    })
  },

  analyze: (id: string): Promise<{ status: string; task_id: string }> => {
    return request<{ status: string; task_id: string }>(`/sites/${id}/analyze`, {
      method: 'POST',
    })
  },

  scanSitemap: (id: string): Promise<ScanStageResponse> => {
    return request<ScanStageResponse>(`/sites/${id}/scan-sitemap`, {
      method: 'POST',
    })
  },

  scanPages: (id: string): Promise<ScanStageResponse> => {
    return request<ScanStageResponse>(`/sites/${id}/scan-pages`, {
      method: 'POST',
    })
  },
}

export const pagesApi = {
  list: (params: PagesQueryParams = {}): Promise<PaginatedResponse<Page>> => {
    const query = buildQueryString({
      site_id: params.site_id,
      kpid: params.kpid,
      imdb_id: params.imdb_id,
      title: params.title,
      year: params.year,
      has_player: params.has_player,
      has_violations: params.has_violations,
      sort_by: params.sort_by,
      sort_order: params.sort_order,
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    })
    return request<PaginatedResponse<Page>>(`/pages${query}`)
  },

  stats: (siteId?: string): Promise<PageStats> => {
    const query = buildQueryString({ site_id: siteId })
    return request<PageStats>(`/pages/stats${query}`)
  },

  exportUrl: (params: PagesQueryParams = {}): string => {
    const query = buildQueryString({
      site_id: params.site_id,
      year: params.year,
      has_player: params.has_player,
      has_violations: params.has_violations,
      sort_by: params.sort_by,
      sort_order: params.sort_order,
    })
    return `${API_BASE}/pages/export${query}`
  },
}

export const tasksApi = {
  list: (params: TasksQueryParams = {}): Promise<PaginatedResponse<ScanTask>> => {
    const query = buildQueryString({
      site_id: params.site_id,
      domain: params.domain,
      status: params.status,
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    })
    return request<PaginatedResponse<ScanTask>>(`/scan-tasks${query}`)
  },

  get: (id: string): Promise<ScanTask> => {
    return request<ScanTask>(`/scan-tasks/${id}`)
  },

  cancel: (taskIds: string[]): Promise<{ cancelled_count: number }> => {
    return request<{ cancelled_count: number }>('/scan-tasks/cancel', {
      method: 'POST',
      body: JSON.stringify({ task_ids: taskIds }),
    })
  },
}

export const contentApi = {
  list: (params: ContentQueryParams = {}): Promise<PaginatedResponse<ContentWithStats>> => {
    const query = buildQueryString({
      title: params.title,
      kinopoisk_id: params.kinopoisk_id,
      imdb_id: params.imdb_id,
      mal_id: params.mal_id,
      shikimori_id: params.shikimori_id,
      mydramalist_id: params.mydramalist_id,
      has_violations: params.has_violations,
      sort_by: params.sort_by,
      sort_order: params.sort_order,
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    })
    return request<PaginatedResponse<ContentWithStats>>(`/content${query}`)
  },

  get: (id: string): Promise<ContentWithStats> => {
    return request<ContentWithStats>(`/content/${id}`)
  },

  create: (data: CreateContentRequest): Promise<ContentWithStats> => {
    return request<ContentWithStats>('/content', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  createBatch: (items: CreateContentRequest[]): Promise<{ created: number; failed: number; content_ids: string[] }> => {
    return request<{ created: number; failed: number; content_ids: string[] }>('/content/batch', {
      method: 'POST',
      body: JSON.stringify({ items }),
    })
  },

  delete: (id: string): Promise<void> => {
    return request<void>(`/content/${id}`, {
      method: 'DELETE',
    })
  },

  deleteBulk: (contentIds: string[]): Promise<{ deleted_count: number }> => {
    return request<{ deleted_count: number }>('/content/delete', {
      method: 'POST',
      body: JSON.stringify({ content_ids: contentIds }),
    })
  },

  violations: (id: string, params: ViolationsQueryParams = {}): Promise<PaginatedResponse<Violation>> => {
    const query = buildQueryString({
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    })
    return request<PaginatedResponse<Violation>>(`/content/${id}/violations${query}`)
  },

  exportViolationsUrl: (id: string): string => {
    return `${API_BASE}/content/${id}/violations/export`
  },

  exportViolationsTextUrl: (id: string): string => {
    return `${API_BASE}/content/${id}/violations/export-text`
  },

  exportUrl: (params?: ContentQueryParams): string => {
    const query = buildQueryString({
      title: params?.title,
      kinopoisk_id: params?.kinopoisk_id,
      imdb_id: params?.imdb_id,
      mal_id: params?.mal_id,
      shikimori_id: params?.shikimori_id,
      mydramalist_id: params?.mydramalist_id,
      has_violations: params?.has_violations,
      sort_by: params?.sort_by,
      sort_order: params?.sort_order,
    })
    return `${API_BASE}/content/export${query}`
  },

  exportAllViolationsTextUrl: (params?: ContentQueryParams): string => {
    const query = buildQueryString({
      title: params?.title,
      kinopoisk_id: params?.kinopoisk_id,
      imdb_id: params?.imdb_id,
      mal_id: params?.mal_id,
      shikimori_id: params?.shikimori_id,
      mydramalist_id: params?.mydramalist_id,
    })
    return `${API_BASE}/content/violations/export-text${query}`
  },

  checkViolations: (contentIds: string[]): Promise<{ checked_count: number }> => {
    return request<{ checked_count: number }>('/content/check-violations', {
      method: 'POST',
      body: JSON.stringify({ content_ids: contentIds }),
    })
  },
}

export const sitemapUrlsApi = {
  list: (
    siteId: string,
    params: { status?: SitemapURLStatus; page?: number; limit?: number } = {}
  ): Promise<SitemapURLsResponse> => {
    const query = buildQueryString({
      status: params.status,
      page: params.page ?? 1,
      limit: params.limit ?? 50,
    })
    return request<SitemapURLsResponse>(`/sites/${siteId}/sitemap-urls${query}`)
  },

  stats: (siteId: string): Promise<SitemapURLStats> => {
    return request<SitemapURLStats>(`/sites/${siteId}/sitemap-urls/stats`)
  },
}

export { ApiError }
