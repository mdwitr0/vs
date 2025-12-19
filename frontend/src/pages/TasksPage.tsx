import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useSearchParams, Link } from 'react-router-dom'
import { tasksApi } from '@/lib/api'
import type { ScanTask, TaskStatus, TaskStage } from '@/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { CopyButton } from '@/components/ui/copy-button'
import { TruncatedText } from '@/components/ui/truncated-text'
import { Input } from '@/components/ui/input'
import { Pagination } from '@/components/ui/pagination'
import { PageHeader } from '@/components/PageHeader'
import { useDebouncedValue } from '@/hooks/useDebouncedValue'
import { Square } from 'lucide-react'
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

function formatDate(dateString: string | undefined): string {
  if (!dateString) return '-'
  return new Date(dateString).toLocaleString('ru-RU')
}

function formatDuration(start?: string, end?: string): string {
  if (!start) return '-'
  const startDate = new Date(start)
  const endDate = end ? new Date(end) : new Date() // текущее время для активных задач
  const diffMs = endDate.getTime() - startDate.getTime()
  const seconds = Math.floor(diffMs / 1000)
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  return `${minutes}m ${remainingSeconds}s`
}

function StatusBadge({ status }: { status: TaskStatus }) {
  const variants: Record<TaskStatus, 'secondary' | 'default' | 'outline' | 'destructive'> = {
    pending: 'secondary',
    processing: 'default',
    completed: 'outline',
    failed: 'destructive',
    cancelled: 'secondary',
  }
  const labels: Record<TaskStatus, string> = {
    pending: 'Ожидает',
    processing: 'Выполняется',
    completed: 'Завершено',
    failed: 'Ошибка',
    cancelled: 'Отменено',
  }
  if (!status || !labels[status]) return null
  return <Badge variant={variants[status]}>{labels[status]}</Badge>
}

function StageBadge({ stage }: { stage: TaskStage }) {
  const labels: Record<TaskStage, string> = {
    sitemap: 'Сбор URL',
    page: 'Парсинг',
    done: 'Завершено',
  }
  const variants: Record<TaskStage, 'secondary' | 'outline' | 'default'> = {
    sitemap: 'outline',
    page: 'secondary',
    done: 'default',
  }
  if (!stage || !labels[stage]) return null
  return <Badge variant={variants[stage]}>{labels[stage]}</Badge>
}

const STATUS_OPTIONS: { value: string; label: string }[] = [
  { value: 'all', label: 'Все статусы' },
  { value: 'pending', label: 'Ожидает' },
  { value: 'processing', label: 'Выполняется' },
  { value: 'completed', label: 'Завершено' },
  { value: 'failed', label: 'Ошибка' },
  { value: 'cancelled', label: 'Отменено' },
]


export function TasksPage() {
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()

  // Читаем параметры из URL
  const domainFilter = searchParams.get('domain') || ''
  const statusFilter = searchParams.get('status') || 'all'
  const currentPage = parseInt(searchParams.get('page') || '1', 10)
  const pageSize = parseInt(searchParams.get('size') || '20', 10)

  const debouncedDomain = useDebouncedValue(domainFilter, 300)
  const [selectedTasks, setSelectedTasks] = useState<Set<string>>(new Set())
  const [, setTick] = useState(0) // для перерендера длительности активных задач

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

  const setDomainFilter = (value: string) => {
    updateParams({ domain: value || undefined, page: '1' })
  }

  const setStatusFilter = (value: string) => {
    updateParams({ status: value === 'all' ? undefined : value, page: '1' })
  }

  const setCurrentPage = (page: number) => {
    updateParams({ page: page > 1 ? String(page) : undefined })
  }

  const setPageSize = (size: number) => {
    updateParams({ size: size !== 20 ? String(size) : undefined, page: undefined })
  }

  const tasksQuery = useQuery({
    queryKey: ['tasks', {
      domain: debouncedDomain || undefined,
      status: statusFilter === 'all' ? undefined : statusFilter,
      limit: pageSize,
      offset: (currentPage - 1) * pageSize
    }],
    queryFn: () => tasksApi.list({
      domain: debouncedDomain || undefined,
      status: statusFilter === 'all' ? undefined : (statusFilter as TaskStatus),
      limit: pageSize,
      offset: (currentPage - 1) * pageSize
    }),
    refetchInterval: 5000,
  })

  const cancelMutation = useMutation({
    mutationFn: tasksApi.cancel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tasks'] })
      setSelectedTasks(new Set())
    },
  })

  const tasks = tasksQuery.data?.items ?? []
  const total = tasksQuery.data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)

  const handlePageChange = (page: number) => {
    setCurrentPage(page)
    setSelectedTasks(new Set())
  }

  const handlePageSizeChange = (size: number) => {
    setPageSize(size)
    setSelectedTasks(new Set())
  }

  const activeTasks = tasks.filter(
    (t) => t.status === 'pending' || t.status === 'processing'
  )
  const selectedActiveTaskIds = [...selectedTasks].filter((id) =>
    activeTasks.some((t) => t.id === id)
  )

  // Обновляем длительность активных задач каждые 5 секунд
  useEffect(() => {
    if (activeTasks.length === 0) return
    const interval = setInterval(() => setTick((t) => t + 1), 5000)
    return () => clearInterval(interval)
  }, [activeTasks.length])

  const toggleTask = (taskId: string) => {
    const newSet = new Set(selectedTasks)
    if (newSet.has(taskId)) {
      newSet.delete(taskId)
    } else {
      newSet.add(taskId)
    }
    setSelectedTasks(newSet)
  }

  const toggleAll = () => {
    if (selectedActiveTaskIds.length === activeTasks.length) {
      setSelectedTasks(new Set())
    } else {
      setSelectedTasks(new Set(activeTasks.map((t) => t.id)))
    }
  }

  const handleCancel = () => {
    if (selectedActiveTaskIds.length > 0) {
      cancelMutation.mutate(selectedActiveTaskIds)
    }
  }

  const isActive = (status: TaskStatus) =>
    status === 'pending' || status === 'processing'

  const clearAllFilters = () => {
    updateParams({
      domain: undefined,
      status: undefined,
      page: undefined,
    })
  }

  const activeFiltersCount = [
    domainFilter,
    statusFilter !== 'all' ? statusFilter : '',
  ].filter(Boolean).length

  const hasActiveFilters = !!(domainFilter || statusFilter !== 'all')

  const filtersContent = (
    <>
      <div className="grid grid-cols-2 gap-2">
        <Input
          placeholder="Поиск по домену..."
          value={domainFilter}
          onChange={(e) => setDomainFilter(e.target.value)}
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
        title="Задачи сканирования"
        filters={filtersContent}
        actions={[
          {
            label: cancelMutation.isPending ? 'Отмена...' : `Остановить (${selectedActiveTaskIds.length})`,
            onClick: handleCancel,
            disabled: selectedActiveTaskIds.length === 0 || cancelMutation.isPending,
            variant: 'destructive',
            icon: <Square className="h-4 w-4" />,
          },
        ]}
        activeFiltersCount={activeFiltersCount}
        hasActiveFilters={hasActiveFilters}
      />

      {tasksQuery.isLoading && (
        <p className="text-muted-foreground">Загрузка...</p>
      )}

      {tasksQuery.isError && (
        <p className="text-destructive">Не удалось загрузить задачи</p>
      )}

      {!tasksQuery.isLoading && !tasksQuery.isError && (
        <Table className="table-fixed">
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <Checkbox
                  checked={
                    activeTasks.length > 0 &&
                    selectedActiveTaskIds.length === activeTasks.length
                  }
                  onCheckedChange={toggleAll}
                  disabled={activeTasks.length === 0}
                />
              </TableHead>
              <TableHead className="w-[200px]">Домен</TableHead>
              <TableHead className="w-[100px]">Этап</TableHead>
              <TableHead className="w-[100px]">Статус</TableHead>
              <TableHead className="w-[100px]">Sitemap</TableHead>
              <TableHead className="w-[120px]">Страницы</TableHead>
              <TableHead className="w-[150px]">Создано</TableHead>
              <TableHead className="w-[100px]">Длительность</TableHead>
              <TableHead className="w-[180px]">Ошибка</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {tasks.map((task: ScanTask) => {
              const sitemapTotal = task.sitemap_result?.total ?? 0
              const pageSuccess = task.page_result?.success ?? 0
              const pageFailed = task.page_result?.failed ?? 0
              const pageTotal = task.page_result?.total ?? 0
              const error = task.sitemap_result?.error || task.page_result?.error
              return (
                <TableRow key={task.id}>
                  <TableCell>
                    <Checkbox
                      checked={selectedTasks.has(task.id)}
                      onCheckedChange={() => toggleTask(task.id)}
                      disabled={!isActive(task.status)}
                    />
                  </TableCell>
                  <TableCell className="font-medium">
                    <div className="flex items-center gap-1">
                      <Link to={`/sites/${task.site_id}`} className="hover:underline">
                        <TruncatedText text={task.domain} maxWidth="max-w-[180px]" />
                      </Link>
                      <CopyButton text={task.domain} />
                    </div>
                  </TableCell>
                  <TableCell>
                    <StageBadge stage={task.stage} />
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={task.status} />
                  </TableCell>
                  <TableCell>{sitemapTotal} URL</TableCell>
                  <TableCell>
                    {pageSuccess}
                    {pageTotal > 0 && ` / ${pageTotal}`}
                    {pageFailed > 0 && (
                      <span className="text-destructive ml-1">
                        ({pageFailed})
                      </span>
                    )}
                  </TableCell>
                  <TableCell>{formatDate(task.created_at)}</TableCell>
                  <TableCell>
                    {formatDuration(task.created_at, task.finished_at ?? undefined)}
                  </TableCell>
                  <TableCell className="text-destructive">
                    {error ? (
                      <TruncatedText text={error} maxWidth="max-w-[180px]" />
                    ) : '-'}
                  </TableCell>
                </TableRow>
              )
            })}
            {tasks.length === 0 && (
              <TableRow>
                <TableCell colSpan={9} className="text-center text-muted-foreground">
                  Задачи не найдены
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      )}

      {!tasksQuery.isLoading && !tasksQuery.isError && total > 0 && (
        <Pagination
          currentPage={currentPage}
          totalPages={totalPages}
          pageSize={pageSize}
          total={total}
          onPageChange={handlePageChange}
          onPageSizeChange={handlePageSizeChange}
        />
      )}

      <p className="text-xs text-muted-foreground">
        Автообновление каждые 5 секунд
      </p>
    </div>
  )
}
