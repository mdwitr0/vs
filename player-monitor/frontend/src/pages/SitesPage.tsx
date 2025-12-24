import { useState, useRef } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { sitesApi } from '@/lib/api'
import type { Site, SiteStatus } from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
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

const STATUS_OPTIONS = [
  { value: 'all', label: 'Все статусы' },
  { value: 'active', label: 'Активен' },
  { value: 'scanning', label: 'Сканирование' },
  { value: 'error', label: 'Ошибка' },
]

export function SitesPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  const fileInputRef = useRef<HTMLInputElement>(null)

  const domainSearch = searchParams.get('domain') || ''
  const statusFilter = searchParams.get('status') || 'all'
  const currentPage = parseInt(searchParams.get('page') || '1', 10)
  const pageSize = 20

  const debouncedDomain = useDebouncedValue(domainSearch, 300)

  const [selectedSites, setSelectedSites] = useState<Set<string>>(new Set())
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [newDomain, setNewDomain] = useState('')

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

  const sitesQuery = useQuery({
    queryKey: ['sites', { domain: debouncedDomain, status: statusFilter, limit: pageSize, offset: (currentPage - 1) * pageSize }],
    queryFn: () => sitesApi.list({
      domain: debouncedDomain || undefined,
      status: statusFilter === 'all' ? undefined : (statusFilter as SiteStatus),
      limit: pageSize,
      offset: (currentPage - 1) * pageSize,
    }),
    refetchInterval: (query) => {
      const hasScanning = query.state.data?.items?.some(s => s.status === 'scanning')
      return hasScanning ? 5000 : false
    },
  })

  const createMutation = useMutation({
    mutationFn: sitesApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sites'] })
      setIsCreateOpen(false)
      setNewDomain('')
    },
  })

  const importMutation = useMutation({
    mutationFn: sitesApi.import,
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ['sites'] })
      alert(`Импортировано: ${result.created}, пропущено: ${result.skipped}`)
    },
  })

  const scanMutation = useMutation({
    mutationFn: (id: string) => sitesApi.scan(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sites'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => sitesApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sites'] })
      setSelectedSites(new Set())
    },
  })

  const sites = sitesQuery.data?.items ?? []
  const total = sitesQuery.data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)

  const handleCreateSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    let domain = newDomain.trim()
    if (!domain) return
    try {
      const parsed = new URL(domain.startsWith('http') ? domain : `https://${domain}`)
      domain = parsed.hostname
    } catch {
      domain = domain.replace(/^https?:\/\//, '').split('/')[0]
    }
    createMutation.mutate(domain)
  }

  const handleImportClick = () => {
    fileInputRef.current?.click()
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) {
      importMutation.mutate(file)
      e.target.value = ''
    }
  }

  const handleRowClick = (siteId: string) => {
    navigate(`/sites/${siteId}`)
  }

  const handleDeleteSelected = async () => {
    if (selectedSites.size === 0) return
    if (!confirm(`Удалить ${selectedSites.size} сайт(ов)?`)) return
    for (const id of selectedSites) {
      await deleteMutation.mutateAsync(id)
    }
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

  const toggleSelectAll = () => {
    if (selectedSites.size === sites.length) {
      setSelectedSites(new Set())
    } else {
      setSelectedSites(new Set(sites.map((s) => s.id)))
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Сайты</h1>
        <div className="flex items-center gap-2">
          {selectedSites.size > 0 && (
            <Button
              variant="destructive"
              size="sm"
              onClick={handleDeleteSelected}
              disabled={deleteMutation.isPending}
            >
              <Trash2 className="h-4 w-4 mr-1" />
              Удалить ({selectedSites.size})
            </Button>
          )}
          <input
            ref={fileInputRef}
            type="file"
            accept=".csv,.txt"
            className="hidden"
            onChange={handleFileChange}
          />
          <Button variant="outline" size="sm" onClick={handleImportClick} disabled={importMutation.isPending}>
            <Upload className="h-4 w-4 mr-1" />
            {importMutation.isPending ? 'Загрузка...' : 'CSV'}
          </Button>
          <Button size="sm" onClick={() => setIsCreateOpen(true)}>
            <Plus className="h-4 w-4 mr-1" />
            Добавить
          </Button>
        </div>
      </div>

      <div className="flex items-center gap-2">
        <Input
          placeholder="Поиск по домену..."
          value={domainSearch}
          onChange={(e) => updateParams({ domain: e.target.value || undefined, page: '1' })}
          className="max-w-xs"
        />
        <Select
          value={statusFilter}
          onValueChange={(v) => updateParams({ status: v === 'all' ? undefined : v, page: '1' })}
        >
          <SelectTrigger className="w-[150px]">
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
      </div>

      {sitesQuery.isLoading && <p className="text-muted-foreground">Загрузка...</p>}
      {sitesQuery.isError && <p className="text-destructive">Ошибка загрузки</p>}

      {!sitesQuery.isLoading && !sitesQuery.isError && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <input
                  type="checkbox"
                  checked={sites.length > 0 && selectedSites.size === sites.length}
                  onChange={toggleSelectAll}
                />
              </TableHead>
              <TableHead>Домен</TableHead>
              <TableHead>Статус</TableHead>
              <TableHead>Страниц</TableHead>
              <TableHead>С плеером</TableHead>
              <TableHead>Без плеера</TableHead>
              <TableHead>Последний обход</TableHead>
              <TableHead className="w-[100px]">Действия</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sites.map((site: Site) => (
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
                  />
                </TableCell>
                <TableCell className="font-medium">{site.domain}</TableCell>
                <TableCell>
                  <StatusBadge status={site.status} />
                </TableCell>
                <TableCell>{site.total_pages}</TableCell>
                <TableCell>
                  <Badge variant="default">{site.pages_with_player}</Badge>
                </TableCell>
                <TableCell>
                  <Badge variant={site.pages_without_player > 0 ? 'destructive' : 'secondary'}>
                    {site.pages_without_player}
                  </Badge>
                </TableCell>
                <TableCell>{formatDate(site.last_scan_at)}</TableCell>
                <TableCell onClick={(e) => e.stopPropagation()}>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => scanMutation.mutate(site.id)}
                    disabled={site.status === 'scanning' || scanMutation.isPending}
                  >
                    <Play className="h-4 w-4" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
            {sites.length === 0 && (
              <TableRow>
                <TableCell colSpan={8} className="text-center text-muted-foreground">
                  Сайты не найдены
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      )}

      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={currentPage === 1}
            onClick={() => updateParams({ page: String(currentPage - 1) })}
          >
            Назад
          </Button>
          <span className="text-sm text-muted-foreground">
            Страница {currentPage} из {totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={currentPage === totalPages}
            onClick={() => updateParams({ page: String(currentPage + 1) })}
          >
            Вперёд
          </Button>
        </div>
      )}

      <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Добавить сайт</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleCreateSubmit}>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="domain">Домен</Label>
                <Input
                  id="domain"
                  placeholder="example.com"
                  value={newDomain}
                  onChange={(e) => setNewDomain(e.target.value)}
                  autoFocus
                />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsCreateOpen(false)}>
                Отмена
              </Button>
              <Button type="submit" disabled={createMutation.isPending}>
                {createMutation.isPending ? 'Добавление...' : 'Добавить'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
