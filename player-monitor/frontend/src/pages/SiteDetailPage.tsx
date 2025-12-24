import { useRef, useEffect } from 'react'
import { useParams, Link, useSearchParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { sitesApi, pagesApi, downloadFile } from '@/lib/api'
import type { Page, PageType, SiteStatus } from '@/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Pagination } from '@/components/ui/pagination'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Download, Play } from 'lucide-react'

function formatDate(dateString: string | undefined): string {
  if (!dateString) return '-'
  return new Date(dateString).toLocaleString('ru-RU')
}

function StatusBadge({ status }: { status: SiteStatus }) {
  const variants: Record<SiteStatus, 'default' | 'destructive' | 'secondary'> = {
    active: 'default',
    scanning: 'secondary',
    error: 'destructive',
  }
  const labels: Record<SiteStatus, string> = {
    active: 'Активен',
    scanning: 'Сканирование...',
    error: 'Ошибка',
  }
  return <Badge variant={variants[status]}>{labels[status]}</Badge>
}

function PageTypeBadge({ type }: { type: PageType }) {
  const labels: Record<PageType, string> = {
    content: 'Контент',
    catalog: 'Каталог',
    static: 'Статика',
    error: 'Ошибка',
  }
  const variants: Record<PageType, 'default' | 'secondary' | 'outline' | 'destructive'> = {
    content: 'default',
    catalog: 'secondary',
    static: 'outline',
    error: 'destructive',
  }
  return <Badge variant={variants[type]}>{labels[type]}</Badge>
}

const PLAYER_OPTIONS = [
  { value: 'all', label: 'Все' },
  { value: 'true', label: 'С плеером' },
  { value: 'false', label: 'Без плеера' },
]

const PAGE_TYPE_OPTIONS = [
  { value: 'all', label: 'Все типы' },
  { value: 'content', label: 'Контент' },
  { value: 'catalog', label: 'Каталог' },
  { value: 'static', label: 'Статика' },
  { value: 'error', label: 'Ошибка' },
]

const EXCLUDE_OPTIONS = [
  { value: 'all', label: 'Все' },
  { value: 'true', label: 'Исключённые' },
  { value: 'false', label: 'В отчёте' },
]

export function SiteDetailPage() {
  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()

  const hasPlayerFilter = searchParams.get('player') || 'all'
  const pageTypeFilter = searchParams.get('type') || 'all'
  const excludeFilter = searchParams.get('exclude') || 'all'
  const currentPage = parseInt(searchParams.get('page') || '1', 10)
  const pageSize = parseInt(searchParams.get('size') || '20', 10)

  const updateParams = (updates: Record<string, string | undefined>) => {
    const newParams = new URLSearchParams(searchParams)
    Object.entries(updates).forEach(([key, value]) => {
      if (value) newParams.set(key, value)
      else newParams.delete(key)
    })
    setSearchParams(newParams, { replace: true })
  }

  const siteQuery = useQuery({
    queryKey: ['site', id],
    queryFn: () => sitesApi.get(id!),
    enabled: !!id,
    refetchInterval: (query) => {
      if (query.state.data?.status === 'scanning') {
        return 2000
      }
      return false
    },
  })

  const pagesQuery = useQuery({
    queryKey: ['site-pages', id, { hasPlayerFilter, pageTypeFilter, excludeFilter, currentPage }],
    queryFn: () => sitesApi.getPages(id!, {
      has_player: hasPlayerFilter === 'all' ? undefined : hasPlayerFilter === 'true',
      page_type: pageTypeFilter === 'all' ? undefined : (pageTypeFilter as PageType),
      exclude_from_report: excludeFilter === 'all' ? undefined : excludeFilter === 'true',
      limit: pageSize,
      offset: (currentPage - 1) * pageSize,
    }),
    enabled: !!id,
  })

  const scanMutation = useMutation({
    mutationFn: () => sitesApi.scan(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['site', id] })
    },
  })

  const excludeMutation = useMutation({
    mutationFn: ({ pageId, exclude }: { pageId: string; exclude: boolean }) =>
      pagesApi.updateExclude(pageId, exclude),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['site-pages', id] })
      queryClient.invalidateQueries({ queryKey: ['site', id] })
    },
  })

  const site = siteQuery.data
  const pages = pagesQuery.data?.items ?? []
  const total = pagesQuery.data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)

  const prevStatus = useRef<SiteStatus | undefined>(undefined)
  useEffect(() => {
    if (prevStatus.current === 'scanning' && site?.status === 'active') {
      queryClient.invalidateQueries({ queryKey: ['site-pages', id] })
    }
    prevStatus.current = site?.status
  }, [site?.status, queryClient, id])

  if (siteQuery.isLoading) {
    return <p className="text-muted-foreground">Загрузка...</p>
  }

  if (siteQuery.isError || !site) {
    return (
      <div className="space-y-4">
        <p className="text-destructive">Не удалось загрузить сайт</p>
        <Link to="/sites">
          <Button variant="outline">Назад к списку</Button>
        </Link>
      </div>
    )
  }

  const handleExport = () => {
    downloadFile(sitesApi.exportPages(id!), `${site.domain}-pages.csv`)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Link to="/sites">
            <Button variant="outline" size="sm">← Назад</Button>
          </Link>
          <h1 className="text-2xl font-semibold">{site.domain}</h1>
          <StatusBadge status={site.status} />
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="icon" onClick={handleExport} disabled={total === 0}>
            <Download className="h-4 w-4" />
          </Button>
          <Button
            onClick={() => scanMutation.mutate()}
            disabled={site.status === 'scanning' || scanMutation.isPending}
          >
            <Play className="h-4 w-4 mr-2" />
            {site.status === 'scanning' ? 'Сканируется...' : 'Сканировать'}
          </Button>
        </div>
      </div>

      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <h2 className="text-xl font-semibold">Страницы</h2>
            <span className="text-sm text-muted-foreground">{total}</span>
            <span className="text-sm">
              Плеер: <span className="text-green-600">{site.pages_with_player}</span>/<span className="text-red-600">{site.pages_without_player}</span>
            </span>
          </div>
          <div className="flex items-center gap-2">
            <Select value={hasPlayerFilter} onValueChange={(v) => updateParams({ player: v === 'all' ? undefined : v, page: undefined })}>
              <SelectTrigger className="w-[140px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PLAYER_OPTIONS.map((opt) => (
                  <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select value={pageTypeFilter} onValueChange={(v) => updateParams({ type: v === 'all' ? undefined : v, page: undefined })}>
              <SelectTrigger className="w-[140px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PAGE_TYPE_OPTIONS.map((opt) => (
                  <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select value={excludeFilter} onValueChange={(v) => updateParams({ exclude: v === 'all' ? undefined : v, page: undefined })}>
              <SelectTrigger className="w-[140px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {EXCLUDE_OPTIONS.map((opt) => (
                  <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        {pagesQuery.isLoading && <p className="text-muted-foreground">Загрузка страниц...</p>}
        {pagesQuery.isError && <p className="text-destructive">Ошибка загрузки страниц</p>}

        {!pagesQuery.isLoading && !pagesQuery.isError && (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>URL</TableHead>
                  <TableHead>Тип</TableHead>
                  <TableHead>Плеер</TableHead>
                  <TableHead>Проверено</TableHead>
                  <TableHead className="w-[150px]">Действия</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pages.map((page: Page) => (
                  <TableRow key={page.id} className={page.exclude_from_report ? 'opacity-50' : ''}>
                    <TableCell>
                      <a
                        href={page.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="hover:underline text-sm truncate block max-w-md"
                      >
                        {page.url}
                      </a>
                    </TableCell>
                    <TableCell>
                      <PageTypeBadge type={page.page_type} />
                    </TableCell>
                    <TableCell>
                      {page.has_player ? (
                        <Badge variant="default">Да</Badge>
                      ) : (
                        <Badge variant="destructive">Нет</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-sm">{formatDate(page.last_checked_at)}</TableCell>
                    <TableCell>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => excludeMutation.mutate({
                          pageId: page.id,
                          exclude: !page.exclude_from_report,
                        })}
                        disabled={excludeMutation.isPending}
                      >
                        {page.exclude_from_report ? 'Включить' : 'Исключить'}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {pages.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-center text-muted-foreground">
                      Страницы не найдены
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>

            {total > 0 && (
              <Pagination
                currentPage={currentPage}
                totalPages={totalPages}
                pageSize={pageSize}
                total={total}
                onPageChange={(page) => updateParams({ page: page > 1 ? String(page) : undefined })}
                onPageSizeChange={(size) => updateParams({ size: size !== 20 ? String(size) : undefined, page: undefined })}
              />
            )}
          </>
        )}
      </div>

      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground border-t pt-4">
        <span>Последний обход: {formatDate(site.last_scan_at)}</span>
      </div>
    </div>
  )
}
