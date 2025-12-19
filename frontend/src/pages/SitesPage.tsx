import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { sitesApi } from '@/lib/api'
import type { Site, SiteStatus, ScannedSinceFilter, HasViolationsFilter, TaskStage, ActiveTaskProgress, LastScanResult } from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Pagination } from '@/components/ui/pagination'
import { CopyButton } from '@/components/ui/copy-button'
import { TruncatedText } from '@/components/ui/truncated-text'
import { PageHeader } from '@/components/PageHeader'
import { useDebouncedValue } from '@/hooks/useDebouncedValue'
import { Upload, Plus, Trash2, Play } from 'lucide-react'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'

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

type StageValue = TaskStage | 'pending_pages' | 'idle'

function StageBadge({ activeStage, progress, pendingUrlsCount, lastScan }: {
  activeStage?: TaskStage
  progress?: ActiveTaskProgress
  pendingUrlsCount?: number
  lastScan?: LastScanResult
}) {
  if (activeStage === 'sitemap') {
    return (
      <div className="flex items-center gap-2">
        <Badge variant="default">Сбор URL</Badge>
        {progress && progress.total > 0 && (
          <span className="text-xs text-muted-foreground">{progress.total}</span>
        )}
      </div>
    )
  }
  if (activeStage === 'page') {
    return (
      <div className="flex items-center gap-2">
        <Badge variant="default">Парсинг</Badge>
        {progress && (
          <span className="text-xs text-muted-foreground">
            {progress.success}
            {progress.total > 0 && ` / ${progress.total}`}
            {progress.failed > 0 && (
              <span className="text-destructive ml-1">({progress.failed})</span>
            )}
          </span>
        )}
      </div>
    )
  }
  if (pendingUrlsCount && pendingUrlsCount > 0) {
    return <Badge variant="outline">Ожидает ({pendingUrlsCount})</Badge>
  }
  if (lastScan) {
    return (
      <div className="flex items-center gap-1">
        <Badge variant={lastScan.status === 'completed' ? 'secondary' : 'destructive'}>
          {lastScan.status === 'completed' ? 'Завершено' : 'Ошибка'}
        </Badge>
        <span className="text-xs text-muted-foreground">
          {lastScan.success}
          {lastScan.total > 0 && lastScan.total !== lastScan.success && ` / ${lastScan.total}`}
        </span>
      </div>
    )
  }
  return <span className="text-muted-foreground">—</span>
}

const STATUS_OPTIONS: { value: string; label: string }[] = [
  { value: 'all', label: 'Все статусы' },
  { value: 'pending', label: 'Анализ' },
  { value: 'active', label: 'Активен' },
  { value: 'down', label: 'Недоступен' },
  { value: 'dead', label: 'Мёртв' },
  { value: 'frozen', label: 'Заморожен' },
  { value: 'moved', label: 'Переехал' },
]

const SCANNED_SINCE_OPTIONS: { value: string; label: string }[] = [
  { value: 'all', label: 'Дата обхода' },
  { value: 'today', label: 'Сегодня' },
  { value: 'week', label: 'За неделю' },
  { value: 'month', label: 'За месяц' },
]

const VIOLATIONS_OPTIONS: { value: string; label: string }[] = [
  { value: 'all', label: 'Нарушения' },
  { value: 'true', label: 'С нарушениями' },
  { value: 'false', label: 'Без нарушений' },
]

const STAGE_OPTIONS: { value: string; label: string }[] = [
  { value: 'all', label: 'Все этапы' },
  { value: 'idle', label: 'Ожидает' },
  { value: 'sitemap', label: 'Сбор URL' },
  { value: 'page', label: 'Парсинг' },
  { value: 'pending_pages', label: 'Ожидает парсинга' },
]

export function SitesPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()

  // Читаем параметры из URL
  const domainSearch = searchParams.get('domain') || ''
  const statusFilter = searchParams.get('status') || 'all'
  const scannedSinceFilter = searchParams.get('scanned_since') || 'all'
  const hasViolationsFilter = searchParams.get('has_violations') || 'all'
  const stageFilter = searchParams.get('stage') || 'all'
  const currentPage = parseInt(searchParams.get('page') || '1', 10)
  const pageSize = parseInt(searchParams.get('size') || '20', 10)

  const debouncedDomain = useDebouncedValue(domainSearch, 300)

  const [selectedSites, setSelectedSites] = useState<Set<string>>(new Set())
  const [isDeleting, setIsDeleting] = useState(false)
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [isCsvOpen, setIsCsvOpen] = useState(false)
  const [csvText, setCsvText] = useState('')
  const [newSiteUrl, setNewSiteUrl] = useState('')

  // Обновляем URL при изменении параметров
  const updateParams = (updates: Record<string, string | undefined>) => {
    const newParams = new URLSearchParams(searchParams)
    Object.entries(updates).forEach(([key, value]) => {
      if (value) {
        newParams.set(key, value)
      } else {
        newParams.delete(key)
      }
    })
    setSearchParams(newParams, { replace: true })
  }

  const setDomainSearch = (value: string) => {
    updateParams({ domain: value || undefined, page: '1' })
  }

  const setStatusFilter = (value: string) => {
    updateParams({ status: value === 'all' ? undefined : value, page: '1' })
  }

  const setScannedSinceFilter = (value: string) => {
    updateParams({ scanned_since: value === 'all' ? undefined : value, page: '1' })
  }

  const setHasViolationsFilter = (value: string) => {
    updateParams({ has_violations: value === 'all' ? undefined : value, page: '1' })
  }

  const setStageFilter = (value: string) => {
    updateParams({ stage: value === 'all' ? undefined : value, page: '1' })
  }

  const setCurrentPage = (page: number) => {
    updateParams({ page: page > 1 ? String(page) : undefined })
  }

  const setPageSize = (size: number) => {
    updateParams({ size: size !== 20 ? String(size) : undefined, page: undefined })
  }

  const sitesQuery = useQuery({
    queryKey: ['sites', {
      status: statusFilter === 'all' ? undefined : statusFilter,
      scanned_since: scannedSinceFilter === 'all' ? undefined : scannedSinceFilter,
      has_violations: hasViolationsFilter === 'all' ? undefined : hasViolationsFilter,
      limit: pageSize,
      offset: (currentPage - 1) * pageSize
    }],
    queryFn: () => sitesApi.list({
      status: statusFilter === 'all' ? undefined : (statusFilter as SiteStatus),
      scanned_since: scannedSinceFilter === 'all' ? undefined : (scannedSinceFilter as ScannedSinceFilter),
      has_violations: hasViolationsFilter === 'all' ? undefined : (hasViolationsFilter as HasViolationsFilter),
      limit: pageSize,
      offset: (currentPage - 1) * pageSize,
    }),
    refetchInterval: (query) => {
      // Автообновление если есть сайты с активными задачами
      const hasActive = query.state.data?.items?.some(s => s.active_stage)
      return hasActive ? 5000 : false
    },
  })

  const createMutation = useMutation({
    mutationFn: sitesApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sites'] })
      setIsCreateOpen(false)
      setNewSiteUrl('')
    },
  })

  const scanMutation = useMutation({
    mutationFn: sitesApi.scan,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tasks'] })
      setSelectedSites(new Set())
    },
  })

  const sites = sitesQuery.data?.items ?? []
  const total = sitesQuery.data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)

  // Клиентская фильтрация по домену и этапу
  const filteredSites = sites.filter((s) => {
    // Фильтр по домену
    if (debouncedDomain && !s.domain.toLowerCase().includes(debouncedDomain.toLowerCase())) {
      return false
    }
    // Фильтр по этапу
    if (stageFilter !== 'all') {
      const siteStage = getSiteStage(s)
      if (siteStage !== stageFilter) {
        return false
      }
    }
    return true
  })

  function getSiteStage(site: Site): StageValue {
    if (site.active_stage === 'sitemap') return 'sitemap'
    if (site.active_stage === 'page') return 'page'
    if (site.pending_urls_count && site.pending_urls_count > 0) return 'pending_pages'
    return 'idle'
  }

  const handlePageChange = (page: number) => {
    setCurrentPage(page)
    setSelectedSites(new Set())
  }

  const handlePageSizeChange = (size: number) => {
    setPageSize(size)
    setSelectedSites(new Set())
  }

  const toggleSiteSelection = (siteId: string) => {
    setSelectedSites((prev) => {
      const next = new Set(prev)
      if (next.has(siteId)) {
        next.delete(siteId)
      } else {
        next.add(siteId)
      }
      return next
    })
  }

  const scannableSites = filteredSites.filter((s) => s.status !== 'pending')

  const toggleSelectAll = () => {
    if (selectedSites.size === scannableSites.length) {
      setSelectedSites(new Set())
    } else {
      setSelectedSites(new Set(scannableSites.map((s) => s.id)))
    }
  }

  const handleCreateSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const url = newSiteUrl.trim()
    if (!url) return
    let domain = url
    try {
      const parsed = new URL(url.startsWith('http') ? url : `https://${url}`)
      domain = parsed.hostname
    } catch {
      domain = url.replace(/^https?:\/\//, '').split('/')[0]
    }
    createMutation.mutate({ domain })
  }

  const handleScan = () => {
    if (selectedSites.size === 0) return
    scanMutation.mutate({ site_ids: Array.from(selectedSites) })
  }

  const handleCsvUpload = async () => {
    const lines = csvText
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean)

    const sites = lines
      .map((line) => {
        const [domain, cms, hasSitemap] = line.split(',').map((s) => s.trim())
        return {
          domain,
          cms: cms || undefined,
          has_sitemap: hasSitemap === 'true' || hasSitemap === '1',
        }
      })
      .filter((s) => s.domain)

    if (sites.length > 0) {
      await sitesApi.createBatch(sites)
    }

    queryClient.invalidateQueries({ queryKey: ['sites'] })
    setIsCsvOpen(false)
    setCsvText('')
  }

  const handleRowClick = (siteId: string) => {
    navigate(`/sites/${siteId}`)
  }

  const clearAllFilters = () => {
    updateParams({
      domain: undefined,
      status: undefined,
      scanned_since: undefined,
      has_violations: undefined,
      stage: undefined,
      page: undefined,
    })
  }

  const activeFiltersCount = [
    domainSearch,
    statusFilter !== 'all' ? statusFilter : '',
    scannedSinceFilter !== 'all' ? scannedSinceFilter : '',
    hasViolationsFilter !== 'all' ? hasViolationsFilter : '',
    stageFilter !== 'all' ? stageFilter : '',
  ].filter(Boolean).length

  const hasActiveFilters = !!(domainSearch || statusFilter !== 'all' ||
    scannedSinceFilter !== 'all' || hasViolationsFilter !== 'all' || stageFilter !== 'all')

  const handleDeleteSelected = async () => {
    if (selectedSites.size === 0) return
    if (!confirm(`Удалить ${selectedSites.size} сайт(ов)? Все страницы и задачи будут также удалены.`)) return

    setIsDeleting(true)
    try {
      await sitesApi.deleteBulk([...selectedSites])
      queryClient.invalidateQueries({ queryKey: ['sites'] })
    } finally {
      setIsDeleting(false)
      setSelectedSites(new Set())
    }
  }

  const actions = [
    {
      label: isDeleting ? 'Удаление...' : `Удалить (${selectedSites.size})`,
      onClick: handleDeleteSelected,
      disabled: selectedSites.size === 0 || isDeleting,
      variant: 'destructive' as const,
      icon: <Trash2 className="h-4 w-4" />,
    },
    {
      label: 'Загрузить из CSV',
      onClick: () => setIsCsvOpen(true),
      icon: <Upload className="h-4 w-4" />,
      iconOnly: true,
    },
    {
      label: 'Добавить',
      onClick: () => setIsCreateOpen(true),
      icon: <Plus className="h-4 w-4" />,
    },
    {
      label: scanMutation.isPending ? 'Сканирование...' : `Сканировать (${selectedSites.size})`,
      onClick: handleScan,
      disabled: selectedSites.size === 0 || scanMutation.isPending,
      variant: 'default' as const,
      icon: <Play className="h-4 w-4" />,
    },
  ]

  const filtersContent = (
    <>
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-2">
        <Input
          placeholder="Поиск по домену..."
          value={domainSearch}
          onChange={(e) => setDomainSearch(e.target.value)}
        />
        <Select value={statusFilter} onValueChange={setStatusFilter}>
          <SelectTrigger>
            <SelectValue placeholder="Статус" />
          </SelectTrigger>
          <SelectContent>
            {STATUS_OPTIONS.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={scannedSinceFilter} onValueChange={setScannedSinceFilter}>
          <SelectTrigger>
            <SelectValue placeholder="Дата обхода" />
          </SelectTrigger>
          <SelectContent>
            {SCANNED_SINCE_OPTIONS.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={hasViolationsFilter} onValueChange={setHasViolationsFilter}>
          <SelectTrigger>
            <SelectValue placeholder="Нарушения" />
          </SelectTrigger>
          <SelectContent>
            {VIOLATIONS_OPTIONS.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={stageFilter} onValueChange={setStageFilter}>
          <SelectTrigger>
            <SelectValue placeholder="Этап" />
          </SelectTrigger>
          <SelectContent>
            {STAGE_OPTIONS.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {activeFiltersCount > 0 && (
        <div className="flex items-center">
          <Button variant="ghost" size="sm" onClick={clearAllFilters}>
            Сбросить всё
          </Button>
        </div>
      )}
    </>
  )

  return (
    <div className="space-y-6">
      <PageHeader
        title="Сайты"
        filters={filtersContent}
        actions={actions}
        activeFiltersCount={activeFiltersCount}
        hasActiveFilters={hasActiveFilters}
      />

      {sitesQuery.isLoading && (
        <p className="text-muted-foreground">Загрузка...</p>
      )}

      {sitesQuery.isError && (
        <p className="text-destructive">Не удалось загрузить сайты</p>
      )}

      {!sitesQuery.isLoading && !sitesQuery.isError && (
        <Table className="table-fixed">
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <input
                  type="checkbox"
                  checked={scannableSites.length > 0 && selectedSites.size === scannableSites.length}
                  onChange={toggleSelectAll}
                  disabled={scannableSites.length === 0}
                />
              </TableHead>
              <TableHead className="w-[200px]">Домен</TableHead>
              <TableHead className="w-[100px]">Статус</TableHead>
              <TableHead className="w-[120px]">Этап</TableHead>
              <TableHead className="w-[100px]">Нарушений</TableHead>
              <TableHead className="w-[80px]">CMS</TableHead>
              <TableHead className="w-[80px]">Sitemap</TableHead>
              <TableHead className="w-[150px]">Последняя проверка</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filteredSites.map((site: Site) => (
              <TableRow
                key={site.id}
                className="cursor-pointer hover:bg-muted/50"
                onClick={() => handleRowClick(site.id)}
              >
                <TableCell onClick={(e) => e.stopPropagation()}>
                  <input
                    type="checkbox"
                    checked={selectedSites.has(site.id)}
                    onChange={() => toggleSiteSelection(site.id)}
                    disabled={site.status === 'pending'}
                    title={site.status === 'pending' ? 'Ожидает анализа' : undefined}
                  />
                </TableCell>
                <TableCell className="font-medium">
                  <div className="flex items-center gap-1">
                    <TruncatedText text={site.domain} maxWidth="max-w-[180px]" />
                    <CopyButton text={site.domain} />
                  </div>
                </TableCell>
                <TableCell>
                  <StatusBadge status={site.status} />
                </TableCell>
                <TableCell>
                  <StageBadge
                    activeStage={site.active_stage}
                    progress={site.active_task_progress}
                    pendingUrlsCount={site.pending_urls_count}
                    lastScan={site.last_scan}
                  />
                </TableCell>
                <TableCell>
                  <Badge variant={site.violations_count > 0 ? 'destructive' : 'secondary'}>
                    {site.violations_count}
                  </Badge>
                </TableCell>
                <TableCell className="truncate max-w-[80px]">{site.cms ?? '-'}</TableCell>
                <TableCell>{site.has_sitemap ? 'Да' : 'Нет'}</TableCell>
                <TableCell>{formatDate(site.last_scan_at)}</TableCell>
              </TableRow>
            ))}
            {filteredSites.length === 0 && (
              <TableRow>
                <TableCell colSpan={8} className="text-center text-muted-foreground">
                  Сайты не найдены
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      )}

      {!sitesQuery.isLoading && !sitesQuery.isError && total > 0 && (
        <Pagination
          currentPage={currentPage}
          totalPages={totalPages}
          pageSize={pageSize}
          total={total}
          onPageChange={handlePageChange}
          onPageSizeChange={handlePageSizeChange}
        />
      )}

      <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Добавить сайт</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleCreateSubmit}>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="url">URL сайта</Label>
                <Input
                  id="url"
                  placeholder="https://example.com или example.com"
                  value={newSiteUrl}
                  onChange={(e) => setNewSiteUrl(e.target.value)}
                  autoFocus
                />
                <p className="text-xs text-muted-foreground">
                  CMS и sitemap будут определены автоматически
                </p>
              </div>
            </div>
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setIsCreateOpen(false)}
              >
                Отмена
              </Button>
              <Button type="submit" disabled={createMutation.isPending}>
                {createMutation.isPending ? 'Добавление...' : 'Добавить'}
              </Button>
            </DialogFooter>
            {createMutation.isError && (
              <p className="text-sm text-destructive mt-2">
                Не удалось добавить сайт
              </p>
            )}
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={isCsvOpen} onOpenChange={setIsCsvOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Загрузить CSV</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <p className="text-sm text-muted-foreground">
              Формат: домен или домен,cms,has_sitemap (по одному на строку)
            </p>
            <textarea
              className="w-full min-h-[150px] rounded-md border bg-background px-3 py-2 text-sm font-mono"
              placeholder="example1.com&#10;example2.com,WordPress,true&#10;example3.com,DLE,false"
              value={csvText}
              onChange={(e) => setCsvText(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setIsCsvOpen(false)}
            >
              Отмена
            </Button>
            <Button onClick={handleCsvUpload} disabled={!csvText.trim()}>
              Загрузить
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {scanMutation.isSuccess && (
        <p className="text-sm text-muted-foreground">
          Сканирование запущено для {scanMutation.data.site_count} сайт(ов)
        </p>
      )}
    </div>
  )
}
