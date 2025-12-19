import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { contentApi } from '@/lib/api'
import type { ContentWithStats, CreateContentRequest, ContentSortBy, ContentHasViolations } from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { CopyButton } from '@/components/ui/copy-button'
import { TruncatedText } from '@/components/ui/truncated-text'
import { Checkbox } from '@/components/ui/checkbox'
import { Pagination } from '@/components/ui/pagination'
import { useDebouncedValue } from '@/hooks/useDebouncedValue'
import { PageHeader } from '@/components/PageHeader'
import { Download, Upload, Plus, Trash2, Search } from 'lucide-react'
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

const SORT_OPTIONS: { value: string; label: string }[] = [
  { value: 'violations_count', label: 'По нарушениям' },
  { value: 'created_at', label: 'По дате' },
]

const VIOLATIONS_OPTIONS: { value: string; label: string }[] = [
  { value: 'all', label: 'Все' },
  { value: 'true', label: 'С нарушениями' },
  { value: 'false', label: 'Без нарушений' },
]

function formatDate(dateString: string | undefined): string {
  if (!dateString) return '-'
  return new Date(dateString).toLocaleString('ru-RU')
}

function downloadCsv(data: ContentWithStats[], filename: string) {
  const header = 'Название,Оригинальное название,Год выхода,КиноПоиск ID,IMDb ID,MDL ID,MAL ID,Shikimori ID,Нарушений,Сайтов,Добавлен'
  const rows = data.map((c) =>
    [
      `"${c.title.replace(/"/g, '""')}"`,
      `"${(c.original_title ?? '').replace(/"/g, '""')}"`,
      c.year ?? '',
      c.kinopoisk_id ?? '',
      c.imdb_id ?? '',
      c.mydramalist_id ?? '',
      c.mal_id ?? '',
      c.shikimori_id ?? '',
      c.violations_count,
      c.sites_count,
      c.created_at,
    ].join(',')
  )
  const csv = [header, ...rows].join('\n')
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}

export function ContentPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()

  // Читаем параметры из URL
  const titleSearch = searchParams.get('title') || ''
  const kinopoiskIdFilter = searchParams.get('kinopoisk_id') || ''
  const imdbFilter = searchParams.get('imdb_id') || ''
  const malIdFilter = searchParams.get('mal_id') || ''
  const shikimoriIdFilter = searchParams.get('shikimori_id') || ''
  const mydramalistIdFilter = searchParams.get('mydramalist_id') || ''
  const sortBy = (searchParams.get('sort_by') || 'violations_count') as ContentSortBy
  const hasViolations = searchParams.get('has_violations') || 'all'
  const currentPage = parseInt(searchParams.get('page') || '1', 10)
  const pageSize = parseInt(searchParams.get('size') || '20', 10)

  const debouncedTitle = useDebouncedValue(titleSearch, 300)
  const debouncedKinopoiskId = useDebouncedValue(kinopoiskIdFilter, 300)
  const debouncedImdb = useDebouncedValue(imdbFilter, 300)
  const debouncedMalId = useDebouncedValue(malIdFilter, 300)
  const debouncedShikimoriId = useDebouncedValue(shikimoriIdFilter, 300)
  const debouncedMydramalistId = useDebouncedValue(mydramalistIdFilter, 300)

  const [selectedContent, setSelectedContent] = useState<Set<string>>(new Set())
  const [isChecking, setIsChecking] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [isCsvOpen, setIsCsvOpen] = useState(false)
  const [csvText, setCsvText] = useState('')
  const [newContent, setNewContent] = useState<CreateContentRequest>({
    title: '',
  })

  // Подсчёт активных фильтров
  const activeFiltersCount = [
    titleSearch,
    kinopoiskIdFilter,
    imdbFilter,
    malIdFilter,
    shikimoriIdFilter,
    mydramalistIdFilter,
    hasViolations !== 'all' ? hasViolations : '',
    sortBy !== 'violations_count' ? sortBy : '',
  ].filter(Boolean).length

  const hasActiveFilters = !!(titleSearch || kinopoiskIdFilter || imdbFilter ||
    malIdFilter || shikimoriIdFilter || mydramalistIdFilter ||
    hasViolations !== 'all')

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

  const setTitleSearch = (value: string) => {
    updateParams({ title: value || undefined, page: '1' })
  }

  const setKinopoiskIdFilter = (value: string) => {
    updateParams({ kinopoisk_id: value || undefined, page: '1' })
  }

  const setImdbFilter = (value: string) => {
    updateParams({ imdb_id: value || undefined, page: '1' })
  }

  const setMalIdFilter = (value: string) => {
    updateParams({ mal_id: value || undefined, page: '1' })
  }

  const setShikimoriIdFilter = (value: string) => {
    updateParams({ shikimori_id: value || undefined, page: '1' })
  }

  const setMydramalistIdFilter = (value: string) => {
    updateParams({ mydramalist_id: value || undefined, page: '1' })
  }

  const setSortBy = (value: string) => {
    updateParams({ sort_by: value === 'violations_count' ? undefined : value, page: '1' })
  }

  const setHasViolations = (value: string) => {
    updateParams({ has_violations: value === 'all' ? undefined : value, page: '1' })
  }

  const setCurrentPage = (page: number) => {
    updateParams({ page: page > 1 ? String(page) : undefined })
  }

  const clearAllFilters = () => {
    updateParams({
      title: undefined,
      kinopoisk_id: undefined,
      imdb_id: undefined,
      mal_id: undefined,
      shikimori_id: undefined,
      mydramalist_id: undefined,
      has_violations: undefined,
      sort_by: undefined,
      page: undefined,
    })
  }

  const setPageSize = (size: number) => {
    updateParams({ size: size !== 20 ? String(size) : undefined, page: undefined })
  }

  const contentQuery = useQuery({
    queryKey: ['content', {
      title: debouncedTitle || undefined,
      kinopoisk_id: debouncedKinopoiskId || undefined,
      imdb_id: debouncedImdb || undefined,
      mal_id: debouncedMalId || undefined,
      shikimori_id: debouncedShikimoriId || undefined,
      mydramalist_id: debouncedMydramalistId || undefined,
      has_violations: hasViolations === 'all' ? undefined : hasViolations as ContentHasViolations,
      sort_by: sortBy,
      sort_order: 'desc' as const,
      limit: pageSize,
      offset: (currentPage - 1) * pageSize
    }],
    queryFn: () => contentApi.list({
      title: debouncedTitle || undefined,
      kinopoisk_id: debouncedKinopoiskId || undefined,
      imdb_id: debouncedImdb || undefined,
      mal_id: debouncedMalId || undefined,
      shikimori_id: debouncedShikimoriId || undefined,
      mydramalist_id: debouncedMydramalistId || undefined,
      has_violations: hasViolations === 'all' ? undefined : hasViolations as ContentHasViolations,
      sort_by: sortBy,
      sort_order: 'desc',
      limit: pageSize,
      offset: (currentPage - 1) * pageSize,
    }),
  })

  const createMutation = useMutation({
    mutationFn: contentApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['content'] })
      setIsCreateOpen(false)
      setNewContent({ title: '' })
    },
  })

  const items = contentQuery.data?.items ?? []
  const total = contentQuery.data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)

  const handlePageChange = (page: number) => {
    setCurrentPage(page)
    setSelectedContent(new Set())
  }

  const handlePageSizeChange = (size: number) => {
    setPageSize(size)
    setSelectedContent(new Set())
  }

  const handleCreateSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!newContent.title.trim()) return
    const hasAnyId = newContent.kinopoisk_id || newContent.imdb_id || newContent.mal_id || newContent.shikimori_id || newContent.mydramalist_id
    if (!hasAnyId) return
    createMutation.mutate(newContent)
  }

  const handleCsvUpload = async () => {
    // Парсинг CSV с поддержкой кавычек и многострочных значений
    const parseCsvLine = (line: string): string[] => {
      const result: string[] = []
      let current = ''
      let inQuotes = false

      for (let i = 0; i < line.length; i++) {
        const char = line[i]
        if (char === '"') {
          inQuotes = !inQuotes
        } else if (char === ',' && !inQuotes) {
          result.push(current.trim().replace(/[\r\n]+/g, ' ').trim())
          current = ''
        } else {
          current += char
        }
      }
      result.push(current.trim().replace(/[\r\n]+/g, ' ').trim())
      return result
    }

    const lines = csvText
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean)

    // Пропускаем заголовок если первая строка содержит "Название" или "КП"
    const dataLines = lines[0]?.toLowerCase().includes('название') || lines[0]?.toLowerCase().includes('кп')
      ? lines.slice(1)
      : lines

    // Формат CSV: Название, Оригинальное название, Год выхода, КиноПоиск ID, IMDb ID, MDL ID, MAL ID, Shikimori ID (опционально)
    const items = dataLines
      .map((line) => {
        const [title, originalTitle, year, kinopoiskId, imdbId, mydramalistId, malId, shikimoriId] = parseCsvLine(line)
        return {
          title: title || '',
          original_title: originalTitle || undefined,
          year: year ? parseInt(year, 10) : undefined,
          kinopoisk_id: kinopoiskId || undefined,
          imdb_id: imdbId || undefined,
          mydramalist_id: mydramalistId || undefined,
          mal_id: malId || undefined,
          shikimori_id: shikimoriId || undefined,
        }
      })
      .filter((item) => item.title && (item.kinopoisk_id || item.imdb_id || item.mal_id || item.mydramalist_id || item.shikimori_id))

    if (items.length > 0) {
      await contentApi.createBatch(items)
    }

    queryClient.invalidateQueries({ queryKey: ['content'] })
    setIsCsvOpen(false)
    setCsvText('')
  }

  const handleRowClick = (contentId: string) => {
    navigate(`/content/${contentId}`)
  }

  const toggleContent = (contentId: string) => {
    const newSet = new Set(selectedContent)
    if (newSet.has(contentId)) {
      newSet.delete(contentId)
    } else {
      newSet.add(contentId)
    }
    setSelectedContent(newSet)
  }

  const toggleAll = () => {
    if (selectedContent.size === items.length) {
      setSelectedContent(new Set())
    } else {
      setSelectedContent(new Set(items.map((c) => c.id)))
    }
  }

  const handleCheckViolations = async () => {
    if (selectedContent.size === 0) return
    setIsChecking(true)

    try {
      await contentApi.checkViolations([...selectedContent])
      await contentQuery.refetch()
    } finally {
      setIsChecking(false)
      setSelectedContent(new Set())
    }
  }

  const handleDeleteSelected = async () => {
    if (selectedContent.size === 0) return
    if (!confirm(`Удалить ${selectedContent.size} элемент(ов)?`)) return

    setIsDeleting(true)
    try {
      await contentApi.deleteBulk([...selectedContent])
      queryClient.invalidateQueries({ queryKey: ['content'] })
    } finally {
      setIsDeleting(false)
      setSelectedContent(new Set())
    }
  }

  const actions = [
    {
      label: isChecking ? 'Проверка...' : `Проверить (${selectedContent.size})`,
      onClick: handleCheckViolations,
      disabled: selectedContent.size === 0 || isChecking,
      icon: <Search className="h-4 w-4" />,
    },
    {
      label: isDeleting ? 'Удаление...' : `Удалить (${selectedContent.size})`,
      onClick: handleDeleteSelected,
      disabled: selectedContent.size === 0 || isDeleting,
      variant: 'destructive' as const,
      icon: <Trash2 className="h-4 w-4" />,
    },
    {
      label: 'Выгрузить в CSV',
      onClick: () => downloadCsv(items, 'content.csv'),
      disabled: items.length === 0,
      icon: <Download className="h-4 w-4" />,
      iconOnly: true,
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
      variant: 'default' as const,
      icon: <Plus className="h-4 w-4" />,
    },
  ]

  const filtersContent = (
    <>
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-2">
        <Input
          placeholder="Название..."
          value={titleSearch}
          onChange={(e) => setTitleSearch(e.target.value)}
        />
        <Input
          placeholder="KP ID"
          value={kinopoiskIdFilter}
          onChange={(e) => setKinopoiskIdFilter(e.target.value)}
        />
        <Input
          placeholder="IMDB"
          value={imdbFilter}
          onChange={(e) => setImdbFilter(e.target.value)}
        />
        <Input
          placeholder="MAL"
          value={malIdFilter}
          onChange={(e) => setMalIdFilter(e.target.value)}
        />
        <Input
          placeholder="Shikimori"
          value={shikimoriIdFilter}
          onChange={(e) => setShikimoriIdFilter(e.target.value)}
        />
        <Input
          placeholder="MDL"
          value={mydramalistIdFilter}
          onChange={(e) => setMydramalistIdFilter(e.target.value)}
        />
      </div>

      <div className="flex items-center gap-2 flex-wrap">
        <Select value={sortBy} onValueChange={setSortBy}>
          <SelectTrigger className="w-40">
            <SelectValue placeholder="Сортировка" />
          </SelectTrigger>
          <SelectContent>
            {SORT_OPTIONS.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={hasViolations} onValueChange={setHasViolations}>
          <SelectTrigger className="w-40">
            <SelectValue placeholder="Нарушения" />
          </SelectTrigger>
          <SelectContent>
            {VIOLATIONS_OPTIONS.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        {activeFiltersCount > 0 && (
          <Button variant="ghost" size="sm" onClick={clearAllFilters}>
            Сбросить всё
          </Button>
        )}
      </div>
    </>
  )

  return (
    <div className="space-y-6">
      <PageHeader
        title="Контент"
        filters={filtersContent}
        actions={actions}
        activeFiltersCount={activeFiltersCount}
        hasActiveFilters={hasActiveFilters}
      />

      {contentQuery.isLoading && (
        <p className="text-muted-foreground">Загрузка...</p>
      )}

      {contentQuery.isError && (
        <p className="text-destructive">Не удалось загрузить контент</p>
      )}

      {!contentQuery.isLoading && !contentQuery.isError && (
        <Table className="table-fixed">
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <Checkbox
                  checked={items.length > 0 && selectedContent.size === items.length}
                  onCheckedChange={toggleAll}
                  disabled={items.length === 0}
                />
              </TableHead>
              <TableHead className="w-[200px]">Название</TableHead>
              <TableHead className="w-[180px]">Оригинальное название</TableHead>
              <TableHead className="w-[60px]">Год</TableHead>
              <TableHead className="w-[180px]">External IDs</TableHead>
              <TableHead className="w-[90px]">Нарушений</TableHead>
              <TableHead className="w-[70px]">Сайтов</TableHead>
              <TableHead className="w-[150px]">Добавлен</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((content: ContentWithStats) => (
              <TableRow
                key={content.id}
                className="cursor-pointer hover:bg-muted/50"
                onClick={() => handleRowClick(content.id)}
              >
                <TableCell onClick={(e) => e.stopPropagation()}>
                  <Checkbox
                    checked={selectedContent.has(content.id)}
                    onCheckedChange={() => toggleContent(content.id)}
                  />
                </TableCell>
                <TableCell className="font-medium">
                  <TruncatedText text={content.title} maxWidth="max-w-[200px]" />
                </TableCell>
                <TableCell>
                  {content.original_title ? (
                    <TruncatedText text={content.original_title} maxWidth="max-w-[180px]" />
                  ) : '-'}
                </TableCell>
                <TableCell>{content.year ?? '-'}</TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1">
                    {content.kinopoisk_id && (
                      <div className="inline-flex items-center gap-1">
                        <a href={`https://kinopoisk.ru/film/${content.kinopoisk_id}`} target="_blank" rel="noopener noreferrer" onClick={(e) => e.stopPropagation()}>
                          <Badge variant="outline" className="hover:bg-muted cursor-pointer">KP: {content.kinopoisk_id}</Badge>
                        </a>
                        <CopyButton text={`https://kinopoisk.ru/film/${content.kinopoisk_id}`} />
                      </div>
                    )}
                    {content.imdb_id && (
                      <div className="inline-flex items-center gap-1">
                        <a href={`https://imdb.com/title/${content.imdb_id}`} target="_blank" rel="noopener noreferrer" onClick={(e) => e.stopPropagation()}>
                          <Badge variant="outline" className="hover:bg-muted cursor-pointer">IMDB: {content.imdb_id}</Badge>
                        </a>
                        <CopyButton text={`https://imdb.com/title/${content.imdb_id}`} />
                      </div>
                    )}
                    {content.mal_id && (
                      <div className="inline-flex items-center gap-1">
                        <a href={`https://myanimelist.net/anime/${content.mal_id}`} target="_blank" rel="noopener noreferrer" onClick={(e) => e.stopPropagation()}>
                          <Badge variant="outline" className="hover:bg-muted cursor-pointer">MAL: {content.mal_id}</Badge>
                        </a>
                        <CopyButton text={`https://myanimelist.net/anime/${content.mal_id}`} />
                      </div>
                    )}
                    {content.shikimori_id && (
                      <div className="inline-flex items-center gap-1">
                        <a href={`https://shikimori.one/animes/${content.shikimori_id}`} target="_blank" rel="noopener noreferrer" onClick={(e) => e.stopPropagation()}>
                          <Badge variant="outline" className="hover:bg-muted cursor-pointer">Shiki: {content.shikimori_id}</Badge>
                        </a>
                        <CopyButton text={`https://shikimori.one/animes/${content.shikimori_id}`} />
                      </div>
                    )}
                    {content.mydramalist_id && (
                      <div className="inline-flex items-center gap-1">
                        <a href={`https://mydramalist.com/${content.mydramalist_id}`} target="_blank" rel="noopener noreferrer" onClick={(e) => e.stopPropagation()}>
                          <Badge variant="outline" className="hover:bg-muted cursor-pointer">MDL: {content.mydramalist_id}</Badge>
                        </a>
                        <CopyButton text={`https://mydramalist.com/${content.mydramalist_id}`} />
                      </div>
                    )}
                  </div>
                </TableCell>
                <TableCell>
                  <Badge variant={content.violations_count > 0 ? 'destructive' : 'secondary'}>
                    {content.violations_count}
                  </Badge>
                </TableCell>
                <TableCell>{content.sites_count}</TableCell>
                <TableCell>{formatDate(content.created_at)}</TableCell>
              </TableRow>
            ))}
            {items.length === 0 && (
              <TableRow>
                <TableCell colSpan={8} className="text-center text-muted-foreground">
                  Контент не найден
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      )}

      {!contentQuery.isLoading && !contentQuery.isError && total > 0 && (
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
            <DialogTitle>Добавить контент</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleCreateSubmit}>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="title">Название</Label>
                <Input
                  id="title"
                  placeholder="Название фильма или сериала"
                  value={newContent.title}
                  onChange={(e) =>
                    setNewContent((prev) => ({ ...prev, title: e.target.value }))
                  }
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="original_title">Оригинальное название (опционально)</Label>
                <Input
                  id="original_title"
                  placeholder="Original title"
                  value={newContent.original_title ?? ''}
                  onChange={(e) =>
                    setNewContent((prev) => ({ ...prev, original_title: e.target.value || undefined }))
                  }
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="year">Год (опционально)</Label>
                <Input
                  id="year"
                  type="number"
                  placeholder="2024"
                  value={newContent.year ?? ''}
                  onChange={(e) =>
                    setNewContent((prev) => ({
                      ...prev,
                      year: e.target.value ? parseInt(e.target.value, 10) : undefined,
                    }))
                  }
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="kinopoisk_id">Kinopoisk ID</Label>
                <Input
                  id="kinopoisk_id"
                  placeholder="12345"
                  value={newContent.kinopoisk_id ?? ''}
                  onChange={(e) =>
                    setNewContent((prev) => ({ ...prev, kinopoisk_id: e.target.value || undefined }))
                  }
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="imdb_id">IMDB ID</Label>
                <Input
                  id="imdb_id"
                  placeholder="tt1234567"
                  value={newContent.imdb_id ?? ''}
                  onChange={(e) =>
                    setNewContent((prev) => ({ ...prev, imdb_id: e.target.value || undefined }))
                  }
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="mal_id">MyAnimeList ID</Label>
                <Input
                  id="mal_id"
                  placeholder="12345"
                  value={newContent.mal_id ?? ''}
                  onChange={(e) =>
                    setNewContent((prev) => ({ ...prev, mal_id: e.target.value || undefined }))
                  }
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="shikimori_id">Shikimori ID</Label>
                <Input
                  id="shikimori_id"
                  placeholder="12345"
                  value={newContent.shikimori_id ?? ''}
                  onChange={(e) =>
                    setNewContent((prev) => ({ ...prev, shikimori_id: e.target.value || undefined }))
                  }
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="mydramalist_id">MyDramaList ID</Label>
                <Input
                  id="mydramalist_id"
                  placeholder="12345"
                  value={newContent.mydramalist_id ?? ''}
                  onChange={(e) =>
                    setNewContent((prev) => ({ ...prev, mydramalist_id: e.target.value || undefined }))
                  }
                />
              </div>
              <p className="text-sm text-muted-foreground">
                Необходимо указать хотя бы один из идентификаторов
              </p>
            </div>
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setIsCreateOpen(false)}
              >
                Отмена
              </Button>
              <Button
                type="submit"
                disabled={createMutation.isPending || !newContent.title || (!newContent.kinopoisk_id && !newContent.imdb_id && !newContent.mal_id && !newContent.shikimori_id && !newContent.mydramalist_id)}
              >
                {createMutation.isPending ? 'Создание...' : 'Создать'}
              </Button>
            </DialogFooter>
            {createMutation.isError && (
              <p className="text-sm text-destructive mt-2">
                Не удалось создать контент
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
              Формат: Название, Оригинальное название, Год выхода, КиноПоиск ID, IMDb ID, MDL ID, MAL ID, Shikimori ID (по одному на строку)
            </p>
            <div className="flex items-center gap-2">
              <input
                type="file"
                accept=".csv,.txt"
                className="hidden"
                id="csv-file-input"
                onChange={(e) => {
                  const file = e.target.files?.[0]
                  if (file) {
                    const reader = new FileReader()
                    reader.onload = (event) => {
                      setCsvText(event.target?.result as string || '')
                    }
                    reader.readAsText(file)
                  }
                  e.target.value = ''
                }}
              />
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => document.getElementById('csv-file-input')?.click()}
              >
                <Upload className="h-4 w-4 mr-2" />
                Выбрать файл
              </Button>
              <span className="text-sm text-muted-foreground">или вставьте данные ниже</span>
            </div>
            <textarea
              className="w-full min-h-[150px] rounded-md border bg-background px-3 py-2 text-sm font-mono"
              placeholder="Фильм 1,Film 1,2024,12345,tt1234567,,,&#10;Аниме,Anime,2023,,,,52991,&#10;Дорама,,2022,,,123,,"
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
    </div>
  )
}
