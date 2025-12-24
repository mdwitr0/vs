import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { auditLogsApi } from '@/lib/api'
import type { AuditLog } from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
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

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString('ru-RU')
}

const ACTION_OPTIONS = [
  { value: 'all', label: 'Все действия' },
  { value: 'domain_upload', label: 'Загрузка доменов' },
  { value: 'settings_change', label: 'Изменение настроек' },
  { value: 'report_export', label: 'Экспорт отчёта' },
  { value: 'user_create', label: 'Создание пользователя' },
  { value: 'user_update', label: 'Обновление пользователя' },
  { value: 'site_scan', label: 'Сканирование сайта' },
  { value: 'site_delete', label: 'Удаление сайта' },
]

function ActionBadge({ action }: { action: string }) {
  const colors: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
    domain_upload: 'default',
    settings_change: 'secondary',
    report_export: 'outline',
    user_create: 'default',
    user_update: 'secondary',
    site_scan: 'default',
    site_delete: 'destructive',
  }
  return <Badge variant={colors[action] || 'outline'}>{action}</Badge>
}

export function AuditLogsPage() {
  const [actionFilter, setActionFilter] = useState('all')
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')
  const [currentPage, setCurrentPage] = useState(1)
  const pageSize = 20

  const logsQuery = useQuery({
    queryKey: ['audit-logs', { actionFilter, dateFrom, dateTo, currentPage }],
    queryFn: () => auditLogsApi.list({
      action: actionFilter === 'all' ? undefined : actionFilter,
      date_from: dateFrom || undefined,
      date_to: dateTo || undefined,
      limit: pageSize,
      offset: (currentPage - 1) * pageSize,
    }),
  })

  const logs = logsQuery.data?.items ?? []
  const total = logsQuery.data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)

  const clearFilters = () => {
    setActionFilter('all')
    setDateFrom('')
    setDateTo('')
    setCurrentPage(1)
  }

  const hasFilters = actionFilter !== 'all' || dateFrom || dateTo

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Журнал действий</h1>

      <div className="flex items-center gap-2 flex-wrap">
        <Select value={actionFilter} onValueChange={(v) => { setActionFilter(v); setCurrentPage(1) }}>
          <SelectTrigger className="w-[180px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {ACTION_OPTIONS.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Input
          type="date"
          value={dateFrom}
          onChange={(e) => { setDateFrom(e.target.value); setCurrentPage(1) }}
          className="w-[150px]"
          placeholder="От"
        />
        <Input
          type="date"
          value={dateTo}
          onChange={(e) => { setDateTo(e.target.value); setCurrentPage(1) }}
          className="w-[150px]"
          placeholder="До"
        />
        {hasFilters && (
          <Button variant="ghost" size="sm" onClick={clearFilters}>
            Сбросить
          </Button>
        )}
      </div>

      {logsQuery.isLoading && <p className="text-muted-foreground">Загрузка...</p>}
      {logsQuery.isError && <p className="text-destructive">Ошибка загрузки</p>}

      {!logsQuery.isLoading && !logsQuery.isError && (
        <>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Дата</TableHead>
                <TableHead>Пользователь</TableHead>
                <TableHead>Действие</TableHead>
                <TableHead>Детали</TableHead>
                <TableHead>IP</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {logs.map((log: AuditLog) => (
                <TableRow key={log.id}>
                  <TableCell className="text-sm">{formatDate(log.created_at)}</TableCell>
                  <TableCell>{log.user_login || log.user_id}</TableCell>
                  <TableCell>
                    <ActionBadge action={log.action} />
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground max-w-xs truncate">
                    {log.details ? JSON.stringify(log.details) : '-'}
                  </TableCell>
                  <TableCell className="text-sm font-mono">{log.ip_address}</TableCell>
                </TableRow>
              ))}
              {logs.length === 0 && (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-muted-foreground">
                    Записи не найдены
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>

          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={currentPage === 1}
                onClick={() => setCurrentPage(currentPage - 1)}
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
                onClick={() => setCurrentPage(currentPage + 1)}
              >
                Вперёд
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
