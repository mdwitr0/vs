import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { sitesApi, pagesApi, tasksApi, sitemapUrlsApi, downloadFile } from '@/lib/api'
import type { Page, SiteStatus, TaskStatus, PagesQueryParams, SitemapURL, SitemapURLStatus } from '@/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { CopyButton } from '@/components/ui/copy-button'
import { TruncatedText } from '@/components/ui/truncated-text'
import { Pagination } from '@/components/ui/pagination'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Progress } from '@/components/ui/progress'
import { Input } from '@/components/ui/input'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { ChevronDown, Filter, Download } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useState } from 'react'

function formatDate(dateString: string | undefined): string {
  if (!dateString) return '-'
  return new Date(dateString).toLocaleString('ru-RU')
}

function StatusBadge({ status }: { status: SiteStatus }) {
  const variants: Record<SiteStatus, 'default' | 'destructive' | 'outline' | 'secondary'> = {
    pending: 'secondary',
    active: 'default',
    down: 'destructive',
    dead: 'outline',
    frozen: 'secondary',
    moved: 'outline',
  }
  const labels: Record<SiteStatus, string> = {
    pending: 'Анализ...',
    active: 'Активен',
    down: 'Недоступен',
    dead: 'Мёртв',
    frozen: 'Заморожен',
    moved: 'Переехал',
  }
  if (!status || !labels[status]) return null
  return <Badge variant={variants[status]}>{labels[status]}</Badge>
}

function TaskStatusBadge({ status }: { status: TaskStatus }) {
  const variants: Record<TaskStatus, 'default' | 'destructive' | 'outline' | 'secondary'> = {
    pending: 'secondary',
    processing: 'default',
    completed: 'outline',
    failed: 'destructive',
    cancelled: 'secondary',
  }
  const labels: Record<TaskStatus, string> = {
    pending: 'Ожидание',
    processing: 'Сканирование',
    completed: 'Завершено',
    failed: 'Ошибка',
    cancelled: 'Отменено',
  }
  if (!status || !labels[status]) return null
  return <Badge variant={variants[status]}>{labels[status]}</Badge>
}

function SitemapStatusBadge({ status }: { status: SitemapURLStatus }) {
  const variants: Record<SitemapURLStatus, 'default' | 'destructive' | 'outline' | 'secondary'> = {
    pending: 'secondary',
    indexed: 'default',
    error: 'destructive',
    skipped: 'outline',
  }
  const labels: Record<SitemapURLStatus, string> = {
    pending: 'Ожидает',
    indexed: 'Проиндексирован',
    error: 'Ошибка',
    skipped: 'Пропущен',
  }
  if (!status || !labels[status]) return null
  return <Badge variant={variants[status]}>{labels[status]}</Badge>
}

type BooleanFilter = '' | 'true' | 'false'

interface PageFilters {
  year: string
  hasPlayer: BooleanFilter
  hasViolations: BooleanFilter
  sortBy: 'indexed_at' | 'year'
  sortOrder: 'asc' | 'desc'
}

const defaultFilters: PageFilters = {
  year: '',
  hasPlayer: '',
  hasViolations: '',
  sortBy: 'indexed_at',
  sortOrder: 'desc',
}

function filterSelectClass(isActive: boolean): string {
  return `h-9 rounded-md border px-3 text-sm ${isActive ? 'bg-primary text-primary-foreground' : 'bg-background'}`
}

function formatFreezeReason(reason: string | undefined): string {
  if (!reason) return 'Неизвестная ошибка'

  if (reason.includes('no such host') || reason.includes('lookup') || reason.includes('DNS')) {
    return 'Домен не найден (DNS ошибка)'
  }
  if (reason.includes('connection refused')) {
    return 'Соединение отклонено сервером'
  }
  if (reason.includes('timeout') || reason.includes('deadline exceeded')) {
    return 'Превышено время ожидания ответа'
  }
  if (reason.includes('connection reset')) {
    return 'Соединение сброшено'
  }
  if (reason.includes('no route to host')) {
    return 'Хост недоступен'
  }
  if (reason.includes('certificate')) {
    return 'Ошибка SSL сертификата'
  }
  if (reason.includes('403') || reason.includes('Forbidden')) {
    return 'Доступ запрещён (403)'
  }
  if (reason.includes('404')) {
    return 'Страница не найдена (404)'
  }
  if (reason.includes('500') || reason.includes('502') || reason.includes('503')) {
    return 'Ошибка сервера'
  }

  return 'Сайт недоступен'
}

export function SiteDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [pagesPage, setPagesPage] = useState(1)
  const [pagesLimit, setPagesLimit] = useState(20)
  const [filters, setFilters] = useState<PageFilters>(defaultFilters)
  const [filtersOpen, setFiltersOpen] = useState(false)
  const [sitemapPage, setSitemapPage] = useState(1)
  const [sitemapLimit, setSitemapLimit] = useState(50)
  const [sitemapStatusFilter, setSitemapStatusFilter] = useState<SitemapURLStatus | ''>('')
  const queryClient = useQueryClient()

  const activeFiltersCount = [
    filters.year,
    filters.hasPlayer,
    filters.hasViolations,
  ].filter(Boolean).length

  const scanMutation = useMutation({
    mutationFn: (force: boolean = false) => sitesApi.scan({ site_ids: [id!], force }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['scan-tasks', id] })
      queryClient.invalidateQueries({ queryKey: ['sitemap-urls-stats', id] })
    },
  })

  const analyzeMutation = useMutation({
    mutationFn: () => sitesApi.analyze(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['site', id] })
    },
  })

  const scanSitemapMutation = useMutation({
    mutationFn: () => sitesApi.scanSitemap(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['scan-tasks', id] })
    },
  })

  const scanPagesMutation = useMutation({
    mutationFn: () => sitesApi.scanPages(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['scan-tasks', id] })
    },
  })

  const siteQuery = useQuery({
    queryKey: ['site', id],
    queryFn: () => sitesApi.get(id!),
    enabled: !!id,
    refetchInterval: (query) => {
      // Auto-refresh while site is pending detection
      if (query.state.data?.status === 'pending') {
        return 5000
      }
      return false
    },
  })

  const pagesQueryParams: PagesQueryParams = {
    site_id: id,
    offset: (pagesPage - 1) * pagesLimit,
    limit: pagesLimit,
    sort_by: filters.sortBy,
    sort_order: filters.sortOrder,
    ...(filters.year && { year: parseInt(filters.year, 10) }),
    ...(filters.hasPlayer === 'true' && { has_player: true }),
    ...(filters.hasViolations === 'true' && { has_violations: true }),
  }

  const pagesQuery = useQuery({
    queryKey: ['pages', pagesQueryParams],
    queryFn: () => pagesApi.list(pagesQueryParams),
    enabled: !!id,
  })


  const taskQuery = useQuery({
    queryKey: ['scan-tasks', id],
    queryFn: () => tasksApi.list({ site_id: id, limit: 1 }),
    enabled: !!id,
    refetchInterval: (query) => {
      const task = query.state.data?.items?.[0]
      if (task && (task.status === 'pending' || task.status === 'processing')) {
        return 3000
      }
      return false
    },
  })

  const sitemapUrlsQuery = useQuery({
    queryKey: ['sitemap-urls', id, sitemapPage, sitemapStatusFilter],
    queryFn: () =>
      sitemapUrlsApi.list(id!, {
        page: sitemapPage,
        limit: sitemapLimit,
        status: sitemapStatusFilter || undefined,
      }),
    enabled: !!id,
  })

  const sitemapStatsQuery = useQuery({
    queryKey: ['sitemap-urls-stats', id],
    queryFn: () => sitemapUrlsApi.stats(id!),
    enabled: !!id,
    refetchInterval: () => {
      // Auto-refresh while page crawl is processing
      const task = taskQuery.data?.items?.[0]
      const pageResult = task?.page_result
      const processing = pageResult?.status === 'pending' || pageResult?.status === 'processing'
      if (processing) {
        return 3000
      }
      return false
    },
  })

  const site = siteQuery.data
  const task = taskQuery.data?.items?.[0]
  const sitemapResult = task?.sitemap_result
  const pageResult = task?.page_result
  const isSitemapProcessing = sitemapResult?.status === 'pending' || sitemapResult?.status === 'processing'
  const isPageProcessing = pageResult?.status === 'pending' || pageResult?.status === 'processing'
  const isScanning = task?.status === 'pending' || task?.status === 'processing'
  const pages = pagesQuery.data?.items ?? []
  const pagesTotal = pagesQuery.data?.total ?? 0
  const pagesTotalPages = Math.ceil(pagesTotal / pagesLimit)

  const sitemapUrls = sitemapUrlsQuery.data?.urls ?? []
  const sitemapTotal = sitemapUrlsQuery.data?.total ?? 0
  const sitemapTotalPages = Math.ceil(sitemapTotal / sitemapLimit)
  const sitemapStats = sitemapStatsQuery.data

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

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Link to="/sites">
            <Button variant="outline" size="sm">
              ← Назад
            </Button>
          </Link>
          <div className="flex items-center gap-2">
            <h1 className="text-2xl font-semibold">{site.domain}</h1>
            <CopyButton text={site.domain} />
          </div>
          <StatusBadge status={site.status} />
        </div>
        <div className="flex items-center gap-2">
          {site.status === 'frozen' && (
            <Button
              variant="outline"
              onClick={() => analyzeMutation.mutate()}
              disabled={analyzeMutation.isPending}
            >
              {analyzeMutation.isPending ? 'Запуск...' : 'Анализ'}
            </Button>
          )}
          <Button
            variant="outline"
            onClick={() => scanMutation.mutate(true)}
            disabled={site.status === 'pending' || site.status === 'frozen' || isScanning || scanMutation.isPending}
            title="Сбросить все страницы и пересканировать заново"
          >
            {scanMutation.isPending ? 'Запуск...' : 'Пересканировать всё'}
          </Button>
          <Button
            onClick={() => scanMutation.mutate(false)}
            disabled={site.status === 'pending' || site.status === 'frozen' || isScanning || scanMutation.isPending}
            title={site.status === 'pending' ? 'Ожидает завершения анализа' : site.status === 'frozen' ? 'Сайт заморожен — запустите анализ' : undefined}
          >
            {site.status === 'pending' ? 'Анализ...' : scanMutation.isPending ? 'Запуск...' : isScanning ? 'Сканируется...' : 'Сканировать'}
          </Button>
        </div>
      </div>

      {/* Analysis Pending Notice */}
      {site.status === 'pending' && (
        <Card className="border-secondary bg-secondary/10">
          <CardContent className="py-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="animate-pulse h-3 w-3 rounded-full bg-secondary" />
                <div>
                  <div className="font-medium">Определение параметров сайта...</div>
                  <div className="text-sm text-muted-foreground">
                    Анализируется CMS, наличие sitemap, тип защиты. Сканирование станет доступно после завершения.
                  </div>
                </div>
              </div>
              {site.failure_count > 0 && (
                <div className="text-sm text-muted-foreground">
                  Попытка {site.failure_count + 1} из 3
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Analysis Failed Notice */}
      {site.status === 'frozen' && site.freeze_reason && (
        <Card className="border-destructive bg-destructive/10">
          <CardContent className="py-4">
            <div className="space-y-1">
              <div className="font-medium">Ошибка анализа сайта</div>
              <div className="text-sm text-muted-foreground">{formatFreezeReason(site.freeze_reason)}</div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Domain Moved Notice */}
      {site.status === 'moved' && site.moved_to_domain && (
        <Card className="border-secondary">
          <CardContent className="py-4">
            <div className="space-y-1">
              <div className="font-medium">Домен переехал</div>
              <div className="text-sm text-muted-foreground flex items-center gap-2">
                <span>Сайт редиректит на</span>
                <Link to={`/sites?domain=${site.moved_to_domain}`} className="underline">
                  {site.moved_to_domain}
                </Link>
                <CopyButton text={site.moved_to_domain} />
              </div>
              {site.moved_at && (
                <div className="text-xs text-muted-foreground">
                  Обнаружено: {formatDate(site.moved_at)}
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Original Domain Notice */}
      {site.original_domain && (
        <Card className="border-secondary">
          <CardContent className="py-4">
            <div className="space-y-1">
              <div className="font-medium">Переехал с другого домена</div>
              <div className="text-sm text-muted-foreground flex items-center gap-2">
                <span>Изначально был</span>
                <Link to={`/sites?domain=${site.original_domain}`} className="underline">
                  {site.original_domain}
                </Link>
                <CopyButton text={site.original_domain} />
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Scan Progress - Two Stages */}
      <Card className={isScanning ? 'border-primary' : ''}>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium">Сканирование</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Stage 1: Sitemap Crawl */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">1. Сбор URL из sitemap</span>
                {sitemapResult && <TaskStatusBadge status={sitemapResult.status} />}
              </div>
              <div className="flex items-center gap-2">
                {sitemapResult && sitemapResult.total > 0 && (
                  <span className="text-sm text-muted-foreground">
                    {sitemapResult.total} URL
                  </span>
                )}
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => scanSitemapMutation.mutate()}
                  disabled={site.status === 'pending' || site.status === 'frozen' || isSitemapProcessing || scanSitemapMutation.isPending}
                >
                  {scanSitemapMutation.isPending ? 'Запуск...' : isSitemapProcessing ? 'Сканируется...' : 'Собрать URL'}
                </Button>
              </div>
            </div>
            {isSitemapProcessing && (
              <div className="text-xs text-muted-foreground">
                Парсинг карты сайта...
              </div>
            )}
            {sitemapResult?.error && (
              <div className="text-sm text-destructive">{sitemapResult.error}</div>
            )}
          </div>

          {/* Stage 2: Page Crawl */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">2. Парсинг страниц</span>
                {pageResult ? (
                  <TaskStatusBadge status={pageResult.status} />
                ) : (
                  <Badge variant="outline">Не начат</Badge>
                )}
              </div>
              <div className="flex items-center gap-2">
                {pageResult && (pageResult.total > 0 || pageResult.success > 0 || pageResult.failed > 0) ? (
                  <span className="text-sm text-muted-foreground">
                    {pageResult.success}
                    {pageResult.total > 0 && ` / ${pageResult.total}`}
                    {pageResult.failed > 0 && (
                      <span className="text-destructive ml-1">
                        ({pageResult.failed} ошибок)
                      </span>
                    )}
                  </span>
                ) : null}
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => scanPagesMutation.mutate()}
                  disabled={
                    site.status === 'pending' ||
                    site.status === 'frozen' ||
                    isPageProcessing ||
                    scanPagesMutation.isPending ||
                    (site.pending_urls_count ?? 0) === 0
                  }
                  title={(site.pending_urls_count ?? 0) === 0 ? 'Нет URL для парсинга. Сначала соберите URL из sitemap.' : undefined}
                >
                  {scanPagesMutation.isPending ? 'Запуск...' : isPageProcessing ? 'Сканируется...' : `Парсить (${site.pending_urls_count ?? 0})`}
                </Button>
              </div>
            </div>
            {isPageProcessing && pageResult && pageResult.total > 0 && (
              <Progress
                value={((pageResult.success + pageResult.failed) / pageResult.total) * 100}
                className="h-2"
              />
            )}
            {isPageProcessing && (!pageResult || pageResult.total === 0) && (
              <div className="text-xs text-muted-foreground">
                Запуск парсинга...
              </div>
            )}
            {pageResult?.error && (
              <div className="text-sm text-destructive">{pageResult.error}</div>
            )}
          </div>

          {/* Timestamps */}
          {task && (
            <div className="flex items-center justify-between text-xs text-muted-foreground pt-2 border-t">
              <span>
                Начато: {formatDate(task.created_at)}
              </span>
              {task.finished_at && !isScanning && (
                <span>Завершено: {formatDate(task.finished_at)}</span>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      <Tabs defaultValue="pages" className="space-y-4">
        <TabsList>
          <TabsTrigger value="pages">Страницы</TabsTrigger>
          <TabsTrigger value="sitemap">Карты сайта</TabsTrigger>
        </TabsList>

        <TabsContent value="pages" className="space-y-4">
          <Collapsible open={filtersOpen} onOpenChange={setFiltersOpen}>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-4">
                <h2 className="text-xl font-semibold">Страницы</h2>
                <span className="text-sm text-muted-foreground">{pagesTotal}</span>
                {site.violations_count > 0 && (
                  <Badge variant="destructive">{site.violations_count} нарушений</Badge>
                )}
              </div>
              <div className="flex items-center gap-2">
                <CollapsibleTrigger asChild>
                  <Button variant="outline" size="sm">
                    <Filter className="h-4 w-4 mr-1" />
                    Фильтры
                    {activeFiltersCount > 0 && (
                      <Badge variant="secondary" className="ml-1">{activeFiltersCount}</Badge>
                    )}
                    <ChevronDown className={cn("h-4 w-4 ml-1 transition-transform", filtersOpen && "rotate-180")} />
                  </Button>
                </CollapsibleTrigger>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="outline"
                      size="icon"
                      onClick={() => downloadFile(pagesApi.exportUrl({
                        site_id: id,
                        year: filters.year ? parseInt(filters.year, 10) : undefined,
                        has_player: filters.hasPlayer === 'true' ? true : filters.hasPlayer === 'false' ? false : undefined,
                        has_violations: filters.hasViolations === 'true' ? true : filters.hasViolations === 'false' ? false : undefined,
                        sort_by: filters.sortBy,
                        sort_order: filters.sortOrder,
                      }), 'pages.csv')}
                      disabled={pagesTotal === 0}
                    >
                      <Download className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Скачать CSV</TooltipContent>
                </Tooltip>
              </div>
            </div>
            <CollapsibleContent>
              <div className="mt-4 p-4 border rounded-lg bg-muted/30">
                <div className="flex items-center gap-2 flex-wrap">
                  <Input
                    type="number"
                    placeholder="Год"
                    value={filters.year}
                    onChange={(e) => {
                      setFilters({ ...filters, year: e.target.value })
                      setPagesPage(1)
                    }}
                    className="w-20 h-9"
                  />
                  <select
                    className={filterSelectClass(filters.hasPlayer !== '')}
                    value={filters.hasPlayer}
                    onChange={(e) => {
                      setFilters({ ...filters, hasPlayer: e.target.value as BooleanFilter })
                      setPagesPage(1)
                    }}
                  >
                    <option value="">Плеер</option>
                    <option value="true">С плеером</option>
                    <option value="false">Без плеера</option>
                  </select>
                  <select
                    className={filterSelectClass(filters.hasViolations !== '')}
                    value={filters.hasViolations}
                    onChange={(e) => {
                      setFilters({ ...filters, hasViolations: e.target.value as BooleanFilter })
                      setPagesPage(1)
                    }}
                  >
                    <option value="">Нарушения</option>
                    <option value="true">С нарушениями</option>
                    <option value="false">Без нарушений</option>
                  </select>
                  <select
                    className="h-9 rounded-md border bg-background px-3 text-sm"
                    value={filters.sortBy}
                    onChange={(e) => {
                      setFilters({ ...filters, sortBy: e.target.value as 'indexed_at' | 'year' })
                      setPagesPage(1)
                    }}
                  >
                    <option value="indexed_at">По дате</option>
                    <option value="year">По году</option>
                  </select>
                  <select
                    className="h-9 rounded-md border bg-background px-3 text-sm"
                    value={filters.sortOrder}
                    onChange={(e) => {
                      setFilters({ ...filters, sortOrder: e.target.value as 'asc' | 'desc' })
                      setPagesPage(1)
                    }}
                  >
                    <option value="desc">По убыв.</option>
                    <option value="asc">По возр.</option>
                  </select>
                </div>
              </div>
            </CollapsibleContent>
          </Collapsible>

          {pagesQuery.isLoading && (
            <p className="text-muted-foreground">Загрузка страниц...</p>
          )}

          {pagesQuery.isError && (
            <p className="text-destructive">Не удалось загрузить страницы</p>
          )}

          {!pagesQuery.isLoading && !pagesQuery.isError && (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>URL / Название</TableHead>
                    <TableHead>Год</TableHead>
                    <TableHead>External IDs</TableHead>
                    <TableHead>Плеер</TableHead>
                    <TableHead>HTTP</TableHead>
                    <TableHead>Проиндексирован</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {pages.map((page: Page) => (
                    <TableRow key={page.id}>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <TruncatedText
                            text={page.title || page.url}
                            as="a"
                            href={page.url}
                            maxWidth="max-w-[300px]"
                            className="font-medium hover:underline"
                          />
                          <CopyButton text={page.url} />
                        </div>
                      </TableCell>
                      <TableCell>{page.year ?? '-'}</TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-1">
                          {page.external_ids.kpid && (
                            <div className="flex items-center gap-1">
                              <Badge variant="outline">KP: {page.external_ids.kpid}</Badge>
                              <CopyButton text={`https://kinopoisk.ru/film/${page.external_ids.kpid}`} />
                            </div>
                          )}
                          {page.external_ids.imdb_id && (
                            <div className="flex items-center gap-1">
                              <Badge variant="outline">IMDB: {page.external_ids.imdb_id}</Badge>
                              <CopyButton text={`https://imdb.com/title/${page.external_ids.imdb_id}`} />
                            </div>
                          )}
                          {!page.external_ids.kpid && !page.external_ids.imdb_id && (
                            <span className="text-muted-foreground">-</span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        {page.player_url ? (
                          <Badge variant="default">Да</Badge>
                        ) : (
                          <span className="text-muted-foreground">-</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={page.http_status === 200 ? 'default' : 'destructive'}
                        >
                          {page.http_status}
                        </Badge>
                      </TableCell>
                      <TableCell>{formatDate(page.indexed_at)}</TableCell>
                    </TableRow>
                  ))}
                  {pages.length === 0 && (
                    <TableRow>
                      <TableCell colSpan={6} className="text-center text-muted-foreground">
                        Страницы не найдены
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>

              {pagesTotal > 0 && (
                <Pagination
                  currentPage={pagesPage}
                  totalPages={pagesTotalPages}
                  pageSize={pagesLimit}
                  total={pagesTotal}
                  onPageChange={setPagesPage}
                  onPageSizeChange={(size) => {
                    setPagesLimit(size)
                    setPagesPage(1)
                  }}
                />
              )}
            </>
          )}
        </TabsContent>

        <TabsContent value="sitemap" className="space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-4">
              <h2 className="text-xl font-semibold">Карты сайта</h2>
              {sitemapStats && (
                <span className="text-sm text-muted-foreground">
                  Всего: {sitemapStats.total} | Ожидает: {sitemapStats.pending} | Проиндексировано: {sitemapStats.indexed} | Ошибок: {sitemapStats.error} | Пропущено: {sitemapStats.skipped}
                </span>
              )}
            </div>
            <div className="flex items-center gap-2">
              <select
                className={filterSelectClass(sitemapStatusFilter !== '')}
                value={sitemapStatusFilter}
                onChange={(e) => {
                  setSitemapStatusFilter(e.target.value as SitemapURLStatus | '')
                  setSitemapPage(1)
                }}
              >
                <option value="">Все статусы</option>
                <option value="pending">Ожидает</option>
                <option value="indexed">Проиндексирован</option>
                <option value="error">Ошибка</option>
                <option value="skipped">Пропущен</option>
              </select>
            </div>
          </div>

          {sitemapUrlsQuery.isLoading && (
            <p className="text-muted-foreground">Загрузка URL...</p>
          )}

          {sitemapUrlsQuery.isError && (
            <p className="text-destructive">Не удалось загрузить URL из карты сайта</p>
          )}

          {!sitemapUrlsQuery.isLoading && !sitemapUrlsQuery.isError && (
            <>
              <Table className="table-fixed">
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-auto">URL</TableHead>
                    <TableHead className="w-[180px]">Источник</TableHead>
                    <TableHead className="w-[140px]">Статус</TableHead>
                    <TableHead className="w-[150px]">Обнаружен</TableHead>
                    <TableHead className="w-[150px]">Проиндексирован</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sitemapUrls.map((item: SitemapURL) => (
                    <TableRow key={item.id}>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <TruncatedText
                            text={item.url}
                            as="a"
                            href={item.url}
                            maxWidth="max-w-[400px]"
                            className="font-medium hover:underline"
                          />
                          <CopyButton text={item.url} />
                        </div>
                      </TableCell>
                      <TableCell>
                        <TruncatedText
                          text={item.sitemap_source}
                          maxWidth="max-w-[150px]"
                          className="text-sm"
                        />
                      </TableCell>
                      <TableCell>
                        <SitemapStatusBadge status={item.status} />
                      </TableCell>
                      <TableCell>{formatDate(item.discovered_at)}</TableCell>
                      <TableCell>{formatDate(item.indexed_at)}</TableCell>
                    </TableRow>
                  ))}
                  {sitemapUrls.length === 0 && (
                    <TableRow>
                      <TableCell colSpan={5} className="text-center text-muted-foreground">
                        URL не найдены
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>

              {sitemapTotal > 0 && (
                <Pagination
                  currentPage={sitemapPage}
                  totalPages={sitemapTotalPages}
                  pageSize={sitemapLimit}
                  total={sitemapTotal}
                  onPageChange={setSitemapPage}
                  onPageSizeChange={(size) => {
                    setSitemapLimit(size)
                    setSitemapPage(1)
                  }}
                />
              )}
            </>
          )}
        </TabsContent>
      </Tabs>

      {/* Site Tech Info - Footer */}
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground border-t pt-4">
        {site.cms && <span>CMS: {site.cms}</span>}
        <span>Sitemap: {site.has_sitemap ? 'да' : 'нет'}</span>
        {site.captcha_type && site.captcha_type !== 'none' && (
          <span>Защита: {site.captcha_type}</span>
        )}
        {site.scanner_type === 'spa' && <span>SPA</span>}
        {site.last_scan_at && <span>Проверка: {formatDate(site.last_scan_at)}</span>}
      </div>
    </div>
  )
}
