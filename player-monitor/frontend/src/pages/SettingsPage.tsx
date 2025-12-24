import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { settingsApi } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

function formatDate(dateString: string | undefined): string {
  if (!dateString) return '-'
  return new Date(dateString).toLocaleString('ru-RU')
}

export function SettingsPage() {
  const queryClient = useQueryClient()
  const [playerPattern, setPlayerPattern] = useState('')
  const [scanInterval, setScanInterval] = useState(24)
  const [patternError, setPatternError] = useState('')

  const settingsQuery = useQuery({
    queryKey: ['settings'],
    queryFn: settingsApi.get,
  })

  const updateMutation = useMutation({
    mutationFn: settingsApi.update,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
    },
  })

  useEffect(() => {
    if (settingsQuery.data) {
      setPlayerPattern(settingsQuery.data.player_pattern || '')
      setScanInterval(settingsQuery.data.scan_interval_hours || 24)
    }
  }, [settingsQuery.data])

  const validatePattern = (pattern: string): boolean => {
    try {
      new RegExp(pattern)
      setPatternError('')
      return true
    } catch (e) {
      setPatternError('Некорректное регулярное выражение')
      return false
    }
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!validatePattern(playerPattern)) return
    updateMutation.mutate({
      player_pattern: playerPattern,
      scan_interval_hours: scanInterval,
    })
  }

  const hasChanges = settingsQuery.data && (
    playerPattern !== settingsQuery.data.player_pattern ||
    scanInterval !== settingsQuery.data.scan_interval_hours
  )

  if (settingsQuery.isLoading) {
    return <p className="text-muted-foreground">Загрузка...</p>
  }

  if (settingsQuery.isError) {
    return <p className="text-destructive">Ошибка загрузки настроек</p>
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Настройки</h1>

      <Card>
        <CardHeader>
          <CardTitle>Параметры сканирования</CardTitle>
          <CardDescription>
            Настройки обнаружения видеоплеера и расписания сканирования
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-6">
            <div className="space-y-2">
              <Label htmlFor="pattern">Паттерн плеера (regex)</Label>
              <Input
                id="pattern"
                value={playerPattern}
                onChange={(e) => {
                  setPlayerPattern(e.target.value)
                  validatePattern(e.target.value)
                }}
                placeholder="iframe.*player|embed.*video"
                className="font-mono"
              />
              {patternError && (
                <p className="text-sm text-destructive">{patternError}</p>
              )}
              <p className="text-sm text-muted-foreground">
                Регулярное выражение для поиска видеоплеера в HTML-коде страницы
              </p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="interval">Интервал сканирования (часы)</Label>
              <Input
                id="interval"
                type="number"
                min={1}
                max={720}
                value={scanInterval}
                onChange={(e) => setScanInterval(parseInt(e.target.value, 10) || 24)}
                className="w-32"
              />
              <p className="text-sm text-muted-foreground">
                Как часто автоматически сканировать сайты (1-720 часов)
              </p>
            </div>

            <div className="flex items-center gap-4">
              <Button type="submit" disabled={updateMutation.isPending || !hasChanges}>
                {updateMutation.isPending ? 'Сохранение...' : 'Сохранить'}
              </Button>
              {updateMutation.isSuccess && (
                <span className="text-sm text-green-600">Сохранено</span>
              )}
              {updateMutation.isError && (
                <span className="text-sm text-destructive">Ошибка сохранения</span>
              )}
            </div>
          </form>
        </CardContent>
      </Card>

      {settingsQuery.data && (
        <div className="text-sm text-muted-foreground">
          Последнее обновление: {formatDate(settingsQuery.data.updated_at)}
        </div>
      )}
    </div>
  )
}
