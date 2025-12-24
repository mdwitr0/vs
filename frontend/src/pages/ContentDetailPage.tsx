import { useParams, Link, useSearchParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { contentApi, downloadFile } from '@/lib/api'
import type { Violation } from '@/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Pagination } from '@/components/ui/pagination'
import { CopyButton } from '@/components/ui/copy-button'
import { Download, FileText, RefreshCw } from 'lucide-react'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useState } from 'react'

function formatDate(dateString: string | undefined): string {
  if (!dateString) return '-'
  return new Date(dateString).toLocaleString('ru-RU')
}

export function ContentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const queryClient = useQueryClient()
  const [isRefreshing, setIsRefreshing] = useState(false)

  const currentPage = parseInt(searchParams.get('page') || '1', 10)
  const pageSize = parseInt(searchParams.get('size') || '20', 10)
  const violationsOffset = (currentPage - 1) * pageSize

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

  const setCurrentPage = (page: number) => {
    updateParams({ page: page > 1 ? String(page) : undefined })
  }

  const setPageSize = (size: number) => {
    updateParams({ size: size !== 20 ? String(size) : undefined, page: undefined })
  }

  const handleRefresh = async () => {
    if (!id) return
    setIsRefreshing(true)
    try {
      await contentApi.checkViolations([id])
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['content', id] }),
        queryClient.invalidateQueries({ queryKey: ['violations', id] }),
      ])
    } finally {
      setIsRefreshing(false)
    }
  }

  const contentQuery = useQuery({
    queryKey: ['content', id],
    queryFn: () => contentApi.get(id!),
    enabled: !!id,
  })

  const violationsQuery = useQuery({
    queryKey: ['violations', id, { offset: violationsOffset, limit: pageSize }],
    queryFn: () => contentApi.violations(id!, { offset: violationsOffset, limit: pageSize }),
    enabled: !!id,
  })

  const content = contentQuery.data
  const violations = violationsQuery.data?.items ?? []
  const violationsTotal = violationsQuery.data?.total ?? 0
  const totalPages = Math.ceil(violationsTotal / pageSize)

  if (contentQuery.isLoading) {
    return <p className="text-muted-foreground">Загрузка...</p>
  }

  if (contentQuery.isError || !content) {
    return (
      <div className="space-y-4">
        <p className="text-destructive">Не удалось загрузить контент</p>
        <Link to="/content">
          <Button variant="outline">Назад к списку</Button>
        </Link>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Link to="/content">
            <Button variant="outline" size="sm">
              ← Назад
            </Button>
          </Link>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-2xl font-semibold">{content.title}</h1>
              {content.year && (
                <span className="text-muted-foreground">({content.year})</span>
              )}
            </div>
            {content.original_title && (
              <p className="text-muted-foreground text-sm">{content.original_title}</p>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2 text-sm">
          <span className="text-muted-foreground">Нарушений:</span>
          <span className={`inline-flex items-center justify-center w-6 h-6 rounded-full text-white ${content.violations_count > 0 ? 'bg-destructive' : 'bg-muted'}`}>
            {content.violations_count}
          </span>
          <span className="text-muted-foreground">на {content.sites_count} сайтах</span>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-3 text-sm">
        {content.kinopoisk_id && (
          <span className="inline-flex items-center gap-1">
            <span className="text-muted-foreground">KP:</span>
            <a href={`https://kinopoisk.ru/film/${content.kinopoisk_id}`} target="_blank" rel="noopener noreferrer" className="hover:underline">{content.kinopoisk_id}</a>
            <CopyButton text={`https://kinopoisk.ru/film/${content.kinopoisk_id}`} />
          </span>
        )}
        {content.imdb_id && (
          <span className="inline-flex items-center gap-1">
            <span className="text-muted-foreground">IMDB:</span>
            <a href={`https://imdb.com/title/${content.imdb_id}`} target="_blank" rel="noopener noreferrer" className="hover:underline">{content.imdb_id}</a>
            <CopyButton text={`https://imdb.com/title/${content.imdb_id}`} />
          </span>
        )}
        {content.mal_id && (
          <span className="inline-flex items-center gap-1">
            <span className="text-muted-foreground">MAL:</span>
            <a href={`https://myanimelist.net/anime/${content.mal_id}`} target="_blank" rel="noopener noreferrer" className="hover:underline">{content.mal_id}</a>
            <CopyButton text={`https://myanimelist.net/anime/${content.mal_id}`} />
          </span>
        )}
        {content.shikimori_id && (
          <span className="inline-flex items-center gap-1">
            <span className="text-muted-foreground">Shikimori:</span>
            <a href={`https://shikimori.one/animes/${content.shikimori_id}`} target="_blank" rel="noopener noreferrer" className="hover:underline">{content.shikimori_id}</a>
            <CopyButton text={`https://shikimori.one/animes/${content.shikimori_id}`} />
          </span>
        )}
        {content.mydramalist_id && (
          <span className="inline-flex items-center gap-1">
            <span className="text-muted-foreground">MDL:</span>
            <a href={`https://mydramalist.com/${content.mydramalist_id}`} target="_blank" rel="noopener noreferrer" className="hover:underline">{content.mydramalist_id}</a>
            <CopyButton text={`https://mydramalist.com/${content.mydramalist_id}`} />
          </span>
        )}
      </div>

      {/* Violations Table */}
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-semibold">Нарушения</h2>
          <TooltipProvider>
            <div className="flex gap-2">
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={handleRefresh}
                    disabled={isRefreshing}
                  >
                    <RefreshCw className={`h-4 w-4 ${isRefreshing ? 'animate-spin' : ''}`} />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Проверить нарушения</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={() => downloadFile(contentApi.exportViolationsUrl(id!), 'violations.csv')}
                    disabled={content.violations_count === 0}
                  >
                    <Download className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Скачать CSV</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={() => downloadFile(contentApi.exportViolationsTextUrl(id!), 'violations.txt')}
                    disabled={content.violations_count === 0}
                  >
                    <FileText className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Скачать отчёт</TooltipContent>
              </Tooltip>
            </div>
          </TooltipProvider>
        </div>

        {violationsQuery.isLoading && (
          <p className="text-muted-foreground">Загрузка нарушений...</p>
        )}

        {violationsQuery.isError && (
          <p className="text-destructive">Не удалось загрузить нарушения</p>
        )}

        {!violationsQuery.isLoading && !violationsQuery.isError && (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Домен</TableHead>
                  <TableHead>URL / Название</TableHead>
                  <TableHead>Совпадение</TableHead>
                  <TableHead>Обнаружено</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {violations.map((violation: Violation) => (
                  <TableRow key={violation.page_id}>
                    <TableCell className="font-medium">
                      <div className="flex items-center gap-1">
                        <span>{violation.domain}</span>
                        <CopyButton text={violation.domain} />
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <a
                          href={violation.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="hover:underline"
                        >
                          {violation.title || violation.url}
                        </a>
                        <CopyButton text={violation.url} />
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">{violation.match_type}</Badge>
                    </TableCell>
                    <TableCell>{formatDate(violation.found_at)}</TableCell>
                  </TableRow>
                ))}
                {violations.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-muted-foreground">
                      Нарушения не найдены
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>

            {violationsTotal > 0 && (
              <Pagination
                currentPage={currentPage}
                totalPages={totalPages}
                pageSize={pageSize}
                total={violationsTotal}
                onPageChange={setCurrentPage}
                onPageSizeChange={setPageSize}
              />
            )}
          </>
        )}
      </div>
    </div>
  )
}
