import type {
  Site,
  Page,
  Settings,
  AuditLog,
  PaginatedResponse,
  SitesQueryParams,
  PagesQueryParams,
  AuditLogsQueryParams,
  User,
  TokenResponse,
  CreateUserRequest,
  UpdateUserRequest,
  UpdateSettingsRequest,
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
    if (value !== undefined && value !== '') {
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
  list: async (): Promise<PaginatedResponse<User>> => {
    const response = await request<PaginatedResponse<User>>('/users')
    return {
      items: response.items || [],
      total: response.total || 0,
    }
  },
  create: (data: CreateUserRequest) =>
    request<User>('/users', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: UpdateUserRequest) =>
    request<User>(`/users/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) =>
    request<void>(`/users/${id}`, { method: 'DELETE' }),
  toggleStatus: (id: string) =>
    request<void>(`/users/${id}/status`, { method: 'PATCH' }),
}

export const sitesApi = {
  list: async (params: SitesQueryParams = {}): Promise<PaginatedResponse<Site>> => {
    const query = buildQueryString({
      domain: params.domain,
      status: params.status,
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    })
    const response = await request<PaginatedResponse<Site>>(`/sites${query}`)
    return {
      items: response.items || [],
      total: response.total || 0,
    }
  },

  get: (id: string): Promise<Site> => {
    return request<Site>(`/sites/${id}`)
  },

  create: (domain: string): Promise<Site> => {
    return request<Site>('/sites', {
      method: 'POST',
      body: JSON.stringify({ domain }),
    })
  },

  import: async (file: File): Promise<{ created: number; skipped: number }> => {
    const token = localStorage.getItem('access_token')
    const formData = new FormData()
    formData.append('file', file)

    const response = await fetch(`${API_BASE}/sites/import`, {
      method: 'POST',
      headers: token ? { 'Authorization': `Bearer ${token}` } : {},
      body: formData,
    })

    if (!response.ok) {
      throw new ApiError(response.status, await response.text())
    }

    return response.json()
  },

  scan: (id: string): Promise<{ message: string }> => {
    return request<{ message: string }>(`/sites/${id}/scan`, {
      method: 'POST',
    })
  },

  delete: (id: string): Promise<{ message: string }> => {
    return request<{ message: string }>(`/sites/${id}`, {
      method: 'DELETE',
    })
  },

  getPages: async (id: string, params: PagesQueryParams = {}): Promise<PaginatedResponse<Page>> => {
    const query = buildQueryString({
      has_player: params.has_player,
      page_type: params.page_type,
      exclude_from_report: params.exclude_from_report,
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    })
    const response = await request<PaginatedResponse<Page>>(`/sites/${id}/pages${query}`)
    return {
      items: response.items || [],
      total: response.total || 0,
    }
  },

  exportPages: (id: string): string => {
    return `${API_BASE}/sites/${id}/export`
  },
}

export const pagesApi = {
  updateExclude: (id: string, exclude: boolean): Promise<void> => {
    return request<void>(`/pages/${id}/exclude`, {
      method: 'PATCH',
      body: JSON.stringify({ exclude }),
    })
  },
}

export const settingsApi = {
  get: (): Promise<Settings> => {
    return request<Settings>('/settings')
  },

  update: (data: UpdateSettingsRequest): Promise<Settings> => {
    return request<Settings>('/settings', {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  },
}

export const auditLogsApi = {
  list: async (params: AuditLogsQueryParams = {}): Promise<PaginatedResponse<AuditLog>> => {
    const query = buildQueryString({
      action: params.action,
      date_from: params.date_from,
      date_to: params.date_to,
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    })
    const response = await request<PaginatedResponse<AuditLog>>(`/audit-logs${query}`)
    return {
      items: response.items || [],
      total: response.total || 0,
    }
  },
}

export { ApiError }
