import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, type StudentTrip } from '@/api/client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { getErrorMessage } from '@/lib/utils'
import {
  AlertTriangle,
  CalendarClock,
  CheckCircle2,
  ExternalLink,
  Image,
  Loader2,
  Plane,
  RefreshCw,
  Search,
  Send,
} from 'lucide-react'
import { toast } from 'sonner'

type BadgeVariant = 'default' | 'secondary' | 'destructive' | 'outline' | 'success' | 'warning'
type TripFilter = 'all' | 'active' | 'needs' | 'completed'

const FILTERS: Array<{ value: TripFilter; label: string }> = [
  { value: 'all', label: 'Все' },
  { value: 'active', label: 'В работе' },
  { value: 'needs', label: 'Требуют внимания' },
  { value: 'completed', label: 'Завершены' },
]

const TRIP_STATUS_LABELS: Record<string, string> = {
  abroad: 'За границей',
  overdue: 'Просрочен',
  returned: 'Вернулся',
  completed: 'Завершено',
}

const STAMP_STATUS_LABELS: Record<string, string> = {
  missing: 'Не загружен',
  submitted: 'Загружен',
  rejected: 'Нужно исправить',
}

export default function StudentTrips() {
  const [trips, setTrips] = useState<StudentTrip[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState<TripFilter>('all')
  const [search, setSearch] = useState('')
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [reason, setReason] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [loadError, setLoadError] = useState('')

  const loadTrips = useCallback(async () => {
    setLoading(true)
    setLoadError('')
    try {
      const data = await api.listStudentTrips()
      const nextTrips = data.trips ?? []
      setTrips(nextTrips)
      setSelectedId((current) => {
        if (current && nextTrips.some((trip) => trip.id === current)) return current
        return nextTrips[0]?.id ?? null
      })
    } catch (error: unknown) {
      const message = getErrorMessage(error)
      setLoadError(message)
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadTrips()
  }, [loadTrips])

  useEffect(() => {
    setReason('')
  }, [selectedId])

  const selectedTrip = useMemo(
    () => trips.find((trip) => trip.id === selectedId) ?? null,
    [selectedId, trips],
  )

  const counts = useMemo(() => {
    return trips.reduce(
      (acc, trip) => {
        acc.total += 1
        if (trip.status === 'completed') acc.completed += 1
        if (isNeedsAttention(trip)) acc.needs += 1
        if (trip.status !== 'completed') acc.active += 1
        return acc
      },
      { total: 0, active: 0, needs: 0, completed: 0 },
    )
  }, [trips])

  const filteredTrips = useMemo(() => {
    const query = search.trim().toLowerCase()
    return trips.filter((trip) => {
      if (filter === 'active' && trip.status === 'completed') return false
      if (filter === 'needs' && !isNeedsAttention(trip)) return false
      if (filter === 'completed' && trip.status !== 'completed') return false
      if (!query) return true

      return (
        String(trip.id).includes(query) ||
        String(trip.user_id).includes(query) ||
        trip.full_name.toLowerCase().includes(query)
      )
    })
  }, [filter, search, trips])

  const handleRequestFix = async () => {
    if (!selectedTrip) return

    setSubmitting(true)
    try {
      const result = await api.requestStudentTripFix(selectedTrip.id, reason.trim())
      setTrips((current) => current.map((trip) => (trip.id === result.trip.id ? result.trip : trip)))
      setReason('')
      if (result.notify_error) {
        toast.warning(`Статус обновлён, но сообщение студенту не ушло: ${result.notify_error}`)
      } else {
        toast.success('Студенту отправлен запрос на исправление')
      }
    } catch (error: unknown) {
      toast.error(getErrorMessage(error))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Поездки студентов</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Заявки на выезд за границу, чеклисты возвращения и фотографии штампов.
          </p>
        </div>
        <Button type="button" variant="outline" onClick={() => void loadTrips()} disabled={loading}>
          {loading ? <Loader2 className="mr-1.5 h-4 w-4 animate-spin" /> : <RefreshCw className="mr-1.5 h-4 w-4" />}
          Обновить
        </Button>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="Всего заявок" value={counts.total} icon={Plane} />
        <StatCard label="В работе" value={counts.active} icon={CalendarClock} />
        <StatCard label="Требуют внимания" value={counts.needs} icon={AlertTriangle} />
        <StatCard label="Завершены" value={counts.completed} icon={CheckCircle2} />
      </div>

      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="relative w-full lg:max-w-sm">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Поиск по ФИО, номеру поездки или user ID"
            className="pl-8"
          />
        </div>
        <div className="flex flex-wrap gap-1.5">
          {FILTERS.map((item) => (
            <Button
              key={item.value}
              type="button"
              variant={filter === item.value ? 'default' : 'outline'}
              size="sm"
              onClick={() => setFilter(item.value)}
            >
              {item.label}
            </Button>
          ))}
        </div>
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_390px]">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Список заявок</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-20">№</TableHead>
                  <TableHead>Студент</TableHead>
                  <TableHead>Даты</TableHead>
                  <TableHead>Статус</TableHead>
                  <TableHead>Штамп</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  Array.from({ length: 5 }).map((_, index) => (
                    <TableRow key={index}>
                      <TableCell><div className="h-4 w-10 animate-pulse rounded bg-muted" /></TableCell>
                      <TableCell><div className="h-4 w-36 animate-pulse rounded bg-muted" /></TableCell>
                      <TableCell><div className="h-4 w-28 animate-pulse rounded bg-muted" /></TableCell>
                      <TableCell><div className="h-5 w-24 animate-pulse rounded-full bg-muted" /></TableCell>
                      <TableCell><div className="h-5 w-24 animate-pulse rounded-full bg-muted" /></TableCell>
                    </TableRow>
                  ))
                ) : loadError ? (
                  <TableRow>
                    <TableCell colSpan={5}>
                      <EmptyState icon={AlertTriangle} title="Не удалось загрузить поездки" text={loadError} />
                    </TableCell>
                  </TableRow>
                ) : filteredTrips.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5}>
                      <EmptyState icon={Plane} title="Поездок не найдено" text="Попробуйте изменить фильтр или строку поиска." />
                    </TableCell>
                  </TableRow>
                ) : (
                  filteredTrips.map((trip) => (
                    <TableRow
                      key={trip.id}
                      data-state={trip.id === selectedId ? 'selected' : undefined}
                      className="cursor-pointer"
                      onClick={() => setSelectedId(trip.id)}
                    >
                      <TableCell className="font-mono text-sm">#{trip.id}</TableCell>
                      <TableCell>
                        <div className="min-w-0">
                          <div className="truncate font-medium">{trip.full_name || 'Без имени'}</div>
                          <div className="text-xs text-muted-foreground">user #{trip.user_id}</div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="text-sm">
                          <div>{formatDate(trip.departure_date)} - {formatDate(trip.expected_return)}</div>
                          {trip.actual_return && (
                            <div className="text-xs text-muted-foreground">Факт: {formatDate(trip.actual_return)}</div>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={tripStatusVariant(trip.status)}>
                          {TRIP_STATUS_LABELS[trip.status] ?? trip.status}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant={stampStatusVariant(trip.stamp_status)}>
                          {STAMP_STATUS_LABELS[trip.stamp_status] ?? trip.stamp_status}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Карточка поездки</CardTitle>
          </CardHeader>
          <CardContent>
            {selectedTrip ? (
              <TripDetails
                trip={selectedTrip}
                reason={reason}
                submitting={submitting}
                onReasonChange={setReason}
                onRequestFix={handleRequestFix}
              />
            ) : (
              <EmptyState icon={Plane} title="Выберите поездку" text="Подробности появятся после выбора строки в таблице." />
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

function TripDetails({
  trip,
  reason,
  submitting,
  onReasonChange,
  onRequestFix,
}: {
  trip: StudentTrip
  reason: string
  submitting: boolean
  onReasonChange: (value: string) => void
  onRequestFix: () => void
}) {
  return (
    <div className="space-y-5">
      <div className="space-y-1.5">
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="outline" className="font-mono">#{trip.id}</Badge>
          <Badge variant={tripStatusVariant(trip.status)}>{TRIP_STATUS_LABELS[trip.status] ?? trip.status}</Badge>
        </div>
        <h2 className="text-lg font-semibold leading-tight">{trip.full_name || 'Без имени'}</h2>
        <p className="text-sm text-muted-foreground">{trip.summary}</p>
      </div>

      <div className="grid grid-cols-2 gap-3 text-sm">
        <InfoItem label="Отъезд" value={formatDate(trip.departure_date)} />
        <InfoItem label="Возврат" value={formatDate(trip.expected_return)} />
        <InfoItem label="Фактический возврат" value={trip.actual_return ? formatDate(trip.actual_return) : '-'} />
        <InfoItem label="Отметка о прибытии" value={trip.arrival_reported_at ? formatDateTime(trip.arrival_reported_at) : '-'} />
      </div>

      <div className="space-y-2">
        <h3 className="text-sm font-medium">Чеклист</h3>
        <div className="divide-y rounded-md border">
          {trip.checklist.map((step) => (
            <div key={step.key} className="flex items-center justify-between gap-3 px-3 py-2.5">
              <span className="min-w-0 text-sm">{step.label}</span>
              <Badge variant={stepStatusVariant(step.status)} className="shrink-0">
                {step.status}
              </Badge>
            </div>
          ))}
        </div>
      </div>

      <div className="space-y-3">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-sm font-medium">Штамп пересечения границы</h3>
          <Badge variant={stampStatusVariant(trip.stamp_status)}>
            {STAMP_STATUS_LABELS[trip.stamp_status] ?? trip.stamp_status}
          </Badge>
        </div>

        {trip.stamp_url ? (
          <div className="space-y-3">
            <a
              href={trip.stamp_url}
              target="_blank"
              rel="noreferrer"
              className="block overflow-hidden rounded-md border bg-muted"
            >
              {trip.stamp_mime_type?.startsWith('image/') ? (
                <img src={trip.stamp_url} alt={`Штамп поездки #${trip.id}`} className="max-h-80 w-full object-contain" />
              ) : (
                <div className="flex min-h-36 flex-col items-center justify-center gap-2 text-sm text-muted-foreground">
                  <Image className="h-8 w-8" />
                  {trip.stamp_file_name || 'Файл штампа'}
                </div>
              )}
            </a>
            <div className="flex items-center justify-between gap-3 text-xs text-muted-foreground">
              <span className="truncate">{trip.stamp_file_name || trip.stamp_file_id}</span>
              <Button type="button" variant="outline" size="sm" asChild>
                <a href={trip.stamp_url} target="_blank" rel="noreferrer">
                  <ExternalLink className="mr-1.5 h-4 w-4" />
                  Открыть
                </a>
              </Button>
            </div>
            {trip.stamp_uploaded_at && (
              <p className="text-xs text-muted-foreground">Загружен: {formatDateTime(trip.stamp_uploaded_at)}</p>
            )}
          </div>
        ) : (
          <div className="flex min-h-28 flex-col items-center justify-center gap-2 rounded-md border border-dashed text-sm text-muted-foreground">
            <Image className="h-7 w-7" />
            Фотография штампа ещё не загружена
          </div>
        )}

        {trip.rejection_reason && (
          <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm">
            <span className="font-medium">Последняя причина исправления: </span>
            {trip.rejection_reason}
          </div>
        )}
      </div>

      <div className="space-y-2">
        <Textarea
          value={reason}
          onChange={(event) => onReasonChange(event.target.value)}
          placeholder="Причина для студента, например: приложите фото именно штампа пересечения границы"
          className="min-h-24"
        />
        <Button type="button" className="w-full" onClick={onRequestFix} disabled={submitting}>
          {submitting ? <Loader2 className="mr-1.5 h-4 w-4 animate-spin" /> : <Send className="mr-1.5 h-4 w-4" />}
          Запросить исправление
        </Button>
      </div>
    </div>
  )
}

function StatCard({
  label,
  value,
  icon: Icon,
}: {
  label: string
  value: number
  icon: typeof Plane
}) {
  return (
    <Card>
      <CardContent className="flex items-center justify-between gap-3 p-4">
        <div>
          <p className="text-xs text-muted-foreground">{label}</p>
          <p className="mt-1 text-2xl font-semibold">{value}</p>
        </div>
        <Icon className="h-5 w-5 text-muted-foreground" />
      </CardContent>
    </Card>
  )
}

function InfoItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border px-3 py-2">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 font-medium">{value}</div>
    </div>
  )
}

function EmptyState({
  icon: Icon,
  title,
  text,
}: {
  icon: typeof Plane
  title: string
  text: string
}) {
  return (
    <div className="flex flex-col items-center justify-center py-10 text-center">
      <Icon className="mb-3 h-10 w-10 text-muted-foreground/40" />
      <p className="text-sm font-medium">{title}</p>
      <p className="mt-1 max-w-sm text-xs text-muted-foreground">{text}</p>
    </div>
  )
}

function isNeedsAttention(trip: StudentTrip) {
  return (
    trip.status === 'overdue' ||
    trip.stamp_status === 'rejected' ||
    Boolean(trip.actual_return && (!trip.stamp_file_id || trip.stamp_status === 'missing'))
  )
}

function tripStatusVariant(status: string): BadgeVariant {
  if (status === 'completed') return 'success'
  if (status === 'overdue') return 'destructive'
  if (status === 'returned') return 'warning'
  return 'secondary'
}

function stampStatusVariant(status: string): BadgeVariant {
  if (status === 'submitted') return 'success'
  if (status === 'rejected') return 'destructive'
  if (status === 'missing') return 'warning'
  return 'outline'
}

function stepStatusVariant(status: string): BadgeVariant {
  const value = status.toLowerCase()
  if (value.includes('сделано') || value.includes('done')) return 'success'
  if (value.includes('просроч') || value.includes('rejected')) return 'destructive'
  if (value.includes('доступ') || value.includes('available')) return 'warning'
  return 'outline'
}

function formatDate(value?: string) {
  if (!value) return '-'
  const trimmed = value.trim()
  const dateOnly = /^(\d{4})-(\d{2})-(\d{2})/.exec(trimmed)
  if (dateOnly) {
    const year = Number(dateOnly[1])
    const month = Number(dateOnly[2])
    const day = Number(dateOnly[3])
    return new Date(year, month - 1, day).toLocaleDateString('ru-RU')
  }
  const date = new Date(trimmed.replace(' ', 'T'))
  if (Number.isNaN(date.getTime())) return trimmed
  return date.toLocaleDateString('ru-RU')
}

function formatDateTime(value?: string) {
  if (!value) return '-'
  const trimmed = value.trim()
  const date = new Date(trimmed.replace(' ', 'T'))
  if (Number.isNaN(date.getTime())) return trimmed
  return date.toLocaleString('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
