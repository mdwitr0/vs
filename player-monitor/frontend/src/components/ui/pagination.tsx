import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

interface PaginationProps {
  currentPage: number
  totalPages: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
  onPageSizeChange: (size: number) => void
}

export function Pagination({
  currentPage,
  totalPages,
  pageSize,
  total,
  onPageChange,
  onPageSizeChange,
}: PaginationProps) {
  const [localPageSize, setLocalPageSize] = useState(pageSize)

  useEffect(() => {
    setLocalPageSize(pageSize)
  }, [pageSize])

  const maxVisiblePages = 5

  const getVisiblePages = () => {
    const pages: (number | 'ellipsis')[] = []

    if (totalPages <= maxVisiblePages + 2) {
      for (let i = 1; i <= totalPages; i++) {
        pages.push(i)
      }
    } else {
      pages.push(1)

      const start = Math.max(2, currentPage - Math.floor(maxVisiblePages / 2))
      const end = Math.min(totalPages - 1, start + maxVisiblePages - 1)

      if (start > 2) {
        pages.push('ellipsis')
      }

      for (let i = start; i <= end; i++) {
        pages.push(i)
      }

      if (end < totalPages - 1) {
        pages.push('ellipsis')
      }

      pages.push(totalPages)
    }

    return pages
  }

  const handlePageSizeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value === '' ? '' : parseInt(e.target.value, 10)
    if (value === '' || (typeof value === 'number' && !isNaN(value))) {
      setLocalPageSize(value as number)
    }
  }

  const applyPageSize = () => {
    if (localPageSize > 0 && localPageSize <= 1000 && localPageSize !== pageSize) {
      onPageSizeChange(localPageSize)
    } else {
      setLocalPageSize(pageSize)
    }
  }

  const handlePageSizeKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      applyPageSize()
      ;(e.target as HTMLInputElement).blur()
    }
  }

  if (totalPages <= 1 && total <= pageSize) {
    return (
      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>Всего: {total}</span>
        <div className="flex items-center gap-2">
          <span>На странице:</span>
          <Input
            type="number"
            min={1}
            max={1000}
            value={localPageSize}
            onChange={handlePageSizeChange}
            onBlur={applyPageSize}
            onKeyDown={handlePageSizeKeyDown}
            className="w-20 h-8"
          />
        </div>
      </div>
    )
  }

  const visiblePages = getVisiblePages()
  const startItem = (currentPage - 1) * pageSize + 1
  const endItem = Math.min(currentPage * pageSize, total)

  return (
    <div className="flex items-center justify-between">
      <span className="text-sm text-muted-foreground">
        {startItem}-{endItem} из {total}
      </span>

      <div className="flex items-center gap-1">
        <Button
          variant="outline"
          size="sm"
          onClick={() => onPageChange(currentPage - 1)}
          disabled={currentPage === 1}
        >
          Назад
        </Button>

        {visiblePages.map((page, index) =>
          page === 'ellipsis' ? (
            <span key={`ellipsis-${index}`} className="px-2">...</span>
          ) : (
            <Button
              key={page}
              variant={page === currentPage ? 'default' : 'outline'}
              size="sm"
              onClick={() => onPageChange(page)}
              className="min-w-[32px]"
            >
              {page}
            </Button>
          )
        )}

        <Button
          variant="outline"
          size="sm"
          onClick={() => onPageChange(currentPage + 1)}
          disabled={currentPage === totalPages}
        >
          Вперёд
        </Button>
      </div>

      <div className="flex items-center gap-2 text-sm">
        <span className="text-muted-foreground">На странице:</span>
        <Input
          type="number"
          min={1}
          max={1000}
          value={localPageSize}
          onChange={handlePageSizeChange}
          onBlur={applyPageSize}
          onKeyDown={handlePageSizeKeyDown}
          className="w-20 h-8"
        />
      </div>
    </div>
  )
}
