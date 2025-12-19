import { useParams, Link, useSearchParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { contentApi } from '@/lib/api'
import type { Violation } from '@/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Pagination } from '@/components/ui/pagination'
import { CopyButton } from '@/components/ui/copy-button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { useState } from 'react'

function formatDate(dateString: string | undefined): string {
  if (!dateString) return '-'
  return new Date(dateString).toLocaleString('ru-RU')
}

function downloadViolationsCsv(contentId: string) {
  window.open(contentApi.exportViolationsUrl(contentId), '_blank')
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
      <div className="flex items-center gap-4">
        <Link to="/content">
          <Button variant="outline" size="sm">
            ← Назад
          </Button>
        </Link>
        <div>
          <h1 className="text-2xl font-semibold">{content.title}</h1>
          {content.original_title && (
            <p className="text-muted-foreground">{content.original_title}</p>
          )}
        </div>
        {content.year && (
          <span className="text-muted-foreground">({content.year})</span>
        )}
      </div>

      {/* Content Info */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Kinopoisk ID
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              {content.kinopoisk_id ? (
                <>
                  <a href={`https://kinopoisk.ru/film/${content.kinopoisk_id}`} target="_blank" rel="noopener noreferrer" className="text-lg font-medium hover:underline">
                    {content.kinopoisk_id}
                  </a>
                  <CopyButton text={`https://kinopoisk.ru/film/${content.kinopoisk_id}`} />
                </>
              ) : (
                <span className="text-lg font-medium">-</span>
              )}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              IMDB ID
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              {content.imdb_id ? (
                <>
                  <a href={`https://imdb.com/title/${content.imdb_id}`} target="_blank" rel="noopener noreferrer" className="text-lg font-medium hover:underline">
                    {content.imdb_id}
                  </a>
                  <CopyButton text={`https://imdb.com/title/${content.imdb_id}`} />
                </>
              ) : (
                <span className="text-lg font-medium">-</span>
              )}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              MyAnimeList ID
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              {content.mal_id ? (
                <>
                  <a href={`https://myanimelist.net/anime/${content.mal_id}`} target="_blank" rel="noopener noreferrer" className="text-lg font-medium hover:underline">
                    {content.mal_id}
                  </a>
                  <CopyButton text={`https://myanimelist.net/anime/${content.mal_id}`} />
                </>
              ) : (
                <span className="text-lg font-medium">-</span>
              )}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Shikimori ID
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              {content.shikimori_id ? (
                <>
                  <a href={`https://shikimori.one/animes/${content.shikimori_id}`} target="_blank" rel="noopener noreferrer" className="text-lg font-medium hover:underline">
                    {content.shikimori_id}
                  </a>
                  <CopyButton text={`https://shikimori.one/animes/${content.shikimori_id}`} />
                </>
              ) : (
                <span className="text-lg font-medium">-</span>
              )}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              MyDramaList ID
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              {content.mydramalist_id ? (
                <>
                  <a href={`https://mydramalist.com/${content.mydramalist_id}`} target="_blank" rel="noopener noreferrer" className="text-lg font-medium hover:underline">
                    {content.mydramalist_id}
                  </a>
                  <CopyButton text={`https://mydramalist.com/${content.mydramalist_id}`} />
                </>
              ) : (
                <span className="text-lg font-medium">-</span>
              )}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Нарушений
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              <Badge variant={content.violations_count > 0 ? 'destructive' : 'secondary'} className="text-lg px-3 py-1">
                {content.violations_count}
              </Badge>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Сайтов с нарушениями
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{content.sites_count}</div>
          </CardContent>
        </Card>
      </div>

      {/* Violations Table */}
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-semibold">Нарушения</h2>
          <div className="flex gap-2">
            <Button
              variant="outline"
              onClick={handleRefresh}
              disabled={isRefreshing}
            >
              {isRefreshing ? 'Обновление...' : 'Проверить нарушения'}
            </Button>
            <Button
              variant="outline"
              onClick={() => downloadViolationsCsv(id!)}
              disabled={content.violations_count === 0}
            >
              Экспорт CSV
            </Button>
          </div>
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
