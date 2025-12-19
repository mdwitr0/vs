import { useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { pagesApi, sitesApi } from '@/lib/api'
import type { Page} from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { CopyButton } from '@/components/ui/copy-button'
import { TruncatedText } from '@/components/ui/truncated-text'
import { Pagination } from '@/components/ui/pagination'
import { useDebouncedValue } from '@/hooks/useDebouncedValue'
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

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString('ru-RU')
}

export function PagesPage() {
  const [searchParams, setSearchParams] = useSearchParams()

  // Читаем параметры из URL
  const titleSearch = searchParams.get('title') || ''
  const kpidFilter = searchParams.get('kpid') || ''
  const imdbFilter = searchParams.get('imdb_id') || ''
  const siteFilter = searchParams.get('site_id') || 'all'
  const currentPage = parseInt(searchParams.get('page') || '1', 10)
  const pageSize = parseInt(searchParams.get('size') || '20', 10)

  const debouncedTitle = useDebouncedValue(titleSearch, 300)
  const debouncedKpid = useDebouncedValue(kpidFilter, 300)
  const debouncedImdb = useDebouncedValue(imdbFilter, 300)

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

  const setKpidFilter = (value: string) => {
    updateParams({ kpid: value || undefined, page: '1' })
  }

  const setImdbFilter = (value: string) => {
    updateParams({ imdb_id: value || undefined, page: '1' })
  }

  const setSiteFilter = (value: string) => {
    updateParams({ site_id: value === 'all' ? undefined : value, page: '1' })
  }

  const setCurrentPage = (page: number) => {
    updateParams({ page: page > 1 ? String(page) : undefined })
  }

  const setPageSize = (size: number) => {
    updateParams({ size: size !== 20 ? String(size) : undefined, page: undefined })
  }

  const pagesQuery = useQuery({
    queryKey: ['pages', {
      title: debouncedTitle || undefined,
      kpid: debouncedKpid || undefined,
      imdb_id: debouncedImdb || undefined,
      site_id: siteFilter === 'all' ? undefined : siteFilter,
      limit: pageSize,
      offset: (currentPage - 1) * pageSize
    }],
    queryFn: () => pagesApi.list({
      title: debouncedTitle || undefined,
      kpid: debouncedKpid || undefined,
      imdb_id: debouncedImdb || undefined,
      site_id: siteFilter === 'all' ? undefined : siteFilter,
      limit: pageSize,
      offset: (currentPage - 1) * pageSize,
    }),
  })

  const sitesQuery = useQuery({
    queryKey: ['sites', { limit: 100 }],
    queryFn: () => sitesApi.list({ limit: 100 }),
  })

  const pages = pagesQuery.data?.items ?? []
  const total = pagesQuery.data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)
  const sites = sitesQuery.data?.items ?? []

  const handlePageChange = (page: number) => {
    setCurrentPage(page)
  }

  const handlePageSizeChange = (size: number) => {
    setPageSize(size)
  }

  const handleReset = () => {
    setSearchParams({}, { replace: true })
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Страницы</h1>
        <div className="flex items-center gap-2">
          <Input
            placeholder="Название..."
            className="w-40"
            value={titleSearch}
            onChange={(e) => setTitleSearch(e.target.value)}
          />
          <Input
            placeholder="Kinopoisk ID"
            className="w-32"
            value={kpidFilter}
            onChange={(e) => setKpidFilter(e.target.value)}
          />
          <Input
            placeholder="IMDB ID"
            className="w-32"
            value={imdbFilter}
            onChange={(e) => setImdbFilter(e.target.value)}
          />
          <Select value={siteFilter} onValueChange={setSiteFilter}>
            <SelectTrigger className="w-48">
              <SelectValue placeholder="Все сайты" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Все сайты</SelectItem>
              {sites.map((site) => (
                <SelectItem key={site.id} value={site.id}>
                  {site.domain}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button variant="outline" onClick={handleReset}>
            Сбросить
          </Button>
        </div>
      </div>

      {pagesQuery.isLoading && (
        <p className="text-muted-foreground">Загрузка...</p>
      )}

      {pagesQuery.isError && (
        <p className="text-destructive">Не удалось загрузить страницы</p>
      )}

      {!pagesQuery.isLoading && !pagesQuery.isError && (
        <>
          <div className="text-sm text-muted-foreground">
            Всего: {total} страниц
          </div>

          <Table className="table-fixed">
            <TableHeader>
              <TableRow>
                <TableHead className="w-[300px]">Название</TableHead>
                <TableHead className="w-[60px]">Год</TableHead>
                <TableHead className="w-[200px]">External IDs</TableHead>
                <TableHead className="w-[80px]">Плеер</TableHead>
                <TableHead className="w-[60px]">HTTP</TableHead>
                <TableHead className="w-[150px]">Индексация</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {pages.map((page: Page) => (
                <TableRow key={page.id}>
                  <TableCell>
                    <div className="flex items-center gap-1">
                      <TruncatedText
                        text={page.title || page.url}
                        as="a"
                        href={page.url}
                        maxWidth="max-w-[280px]"
                        className="font-medium hover:underline"
                      />
                      <CopyButton text={page.url} />
                    </div>
                  </TableCell>
                  <TableCell>{page.year ?? '-'}</TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1">
                      {page.external_ids.kpid && (
                        <div className="inline-flex items-center gap-1">
                          <Badge variant="outline">KP: {page.external_ids.kpid}</Badge>
                          <CopyButton text={`https://kinopoisk.ru/film/${page.external_ids.kpid}`} />
                        </div>
                      )}
                      {page.external_ids.imdb_id && (
                        <div className="inline-flex items-center gap-1">
                          <Badge variant="outline">IMDB: {page.external_ids.imdb_id}</Badge>
                          <CopyButton text={`https://imdb.com/title/${page.external_ids.imdb_id}`} />
                        </div>
                      )}
                      {page.external_ids.tmdb_id && (
                        <Badge variant="outline">TMDB: {page.external_ids.tmdb_id}</Badge>
                      )}
                      {!page.external_ids.kpid &&
                        !page.external_ids.imdb_id &&
                        !page.external_ids.tmdb_id && (
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
        </>
      )}

      {!pagesQuery.isLoading && !pagesQuery.isError && total > 0 && (
        <Pagination
          currentPage={currentPage}
          totalPages={totalPages}
          pageSize={pageSize}
          total={total}
          onPageChange={handlePageChange}
          onPageSizeChange={handlePageSizeChange}
        />
      )}
    </div>
  )
}
