import { useEffect, useState, useMemo } from 'react'
import { Link } from 'react-router-dom'
import { api, PluginInfo } from '@/api/client'
import PluginStatusBadge from '@/components/PluginStatusBadge'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from '@/components/ui/select'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/ui/table'
import { ExternalLink, Globe, Search, Package, Upload, FilterX } from 'lucide-react'
import ChannelStatusCard from '@/components/ChannelStatusCard'
import { HelpTooltip } from '@/components/AdminHelp'

export default function PluginList() {
  const [plugins, setPlugins] = useState<PluginInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState<string>('all')
  const [statusFilter, setStatusFilter] = useState<string>('all')

  useEffect(() => {
    setLoading(true)
    api
      .listPlugins()
      .then(setPlugins)
      .catch((e: Error) => toast.error(e.message))
      .finally(() => setLoading(false))
  }, [])

  const filtered = useMemo(() => {
    const q = search.toLowerCase()
    return plugins.filter((p) => {
      if (typeFilter !== 'all' && p.type !== typeFilter) return false
      if (statusFilter !== 'all' && p.status !== statusFilter) return false
      if (q && !p.name.toLowerCase().includes(q) && !p.id.toLowerCase().includes(q)) return false
      return true
    })
  }, [plugins, typeFilter, statusFilter, search])

  const filtersActive = search !== '' || typeFilter !== 'all' || statusFilter !== 'all'

  return (
    <div>
      {/* Page header */}
      <div className="flex items-start justify-between mb-6">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-2xl font-bold tracking-tight">Плагины</h1>
            <HelpTooltip>
              Плагин - это установленный модуль бота с точками запуска, настройками
              и статусом. Отсюда открываются карточка плагина, права, версии и настройка.
            </HelpTooltip>
          </div>
          <p className="text-muted-foreground mt-1">
            Управление установленными плагинами бота
          </p>
        </div>
        <div className="flex flex-wrap justify-end gap-2">
          <Button asChild>
            <Link to="/admin/plugins/upload">
              <Upload className="h-4 w-4 mr-2" />
              Загрузить плагин
            </Link>
          </Button>
        </div>
      </div>

      <div className="mb-6">
        <ChannelStatusCard />
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-3 mb-2">
        <div className="relative w-full sm:w-64">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Поиск по названию или ID..."
            className="pl-9"
          />
        </div>

        <Select value={typeFilter} onValueChange={setTypeFilter}>
          <SelectTrigger className="w-[150px]">
            <SelectValue placeholder="Все типы" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Все типы</SelectItem>
            <SelectItem value="go">Go</SelectItem>
            <SelectItem value="wasm">Wasm</SelectItem>
          </SelectContent>
        </Select>

        <Select value={statusFilter} onValueChange={setStatusFilter}>
          <SelectTrigger className="w-[170px]">
            <SelectValue placeholder="Все статусы" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Все статусы</SelectItem>
            <SelectItem value="active">Активные</SelectItem>
            <SelectItem value="disabled">Отключённые</SelectItem>
            <SelectItem value="error">С ошибкой</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Result count when filters are active */}
      {!loading && filtersActive && (
        <p className="text-sm text-muted-foreground mb-4">
          Найдено: {filtered.length} из {plugins.length}
        </p>
      )}
      {!loading && !filtersActive && <div className="mb-4" />}

      {/* Loading skeleton */}
      {loading && (
        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Название</TableHead>
                <TableHead className="hidden sm:table-cell">Версия</TableHead>
                <TableHead>Тип</TableHead>
                <TableHead>Статус</TableHead>
                <TableHead className="hidden md:table-cell">Веб-интерфейс</TableHead>
                <TableHead className="text-right hidden sm:table-cell">Триггеры</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell>
                    <Skeleton className="h-4 w-32" />
                  </TableCell>
                  <TableCell className="hidden sm:table-cell">
                    <Skeleton className="h-4 w-12" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-5 w-14 rounded-full" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-5 w-20 rounded-full" />
                  </TableCell>
                  <TableCell className="hidden md:table-cell">
                    <Skeleton className="h-8 w-24 rounded-md" />
                  </TableCell>
                  <TableCell className="text-right hidden sm:table-cell">
                    <Skeleton className="h-4 w-6 ml-auto" />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}

      {/* Empty state: no plugins installed at all */}
      {!loading && plugins.length === 0 && (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-16">
            <div className="rounded-full bg-muted p-4 mb-4">
              <Package className="h-8 w-8 text-muted-foreground" />
            </div>
            <h3 className="text-lg font-semibold mb-1">Плагины не установлены</h3>
            <p className="text-sm text-muted-foreground mb-4 text-center max-w-sm">
              Загрузите первый плагин, чтобы расширить возможности бота
            </p>
            <Button asChild>
              <Link to="/admin/plugins/upload">
                <Upload className="h-4 w-4 mr-2" />
                Загрузить плагин
              </Link>
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Empty state: filters match nothing */}
      {!loading && plugins.length > 0 && filtered.length === 0 && (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-16">
            <div className="rounded-full bg-muted p-4 mb-4">
              <FilterX className="h-8 w-8 text-muted-foreground" />
            </div>
            <h3 className="text-lg font-semibold mb-1">Ничего не найдено</h3>
            <p className="text-sm text-muted-foreground mb-4 text-center max-w-sm">
              Нет плагинов, подходящих под текущие фильтры. Попробуйте изменить параметры поиска.
            </p>
            <Button
              variant="outline"
              onClick={() => {
                setSearch('')
                setTypeFilter('all')
                setStatusFilter('all')
              }}
            >
              Сбросить фильтры
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Plugin table */}
      {!loading && filtered.length > 0 && (
        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Название</TableHead>
                <TableHead className="hidden sm:table-cell">Версия</TableHead>
                <TableHead>Тип</TableHead>
                <TableHead>Статус</TableHead>
                <TableHead className="hidden md:table-cell">Веб-интерфейс</TableHead>
                <TableHead className="text-right hidden sm:table-cell">Триггеры</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((p) => (
                <TableRow key={p.id}>
                  <TableCell>
                    <div className="min-w-0 space-y-1">
                      <div className="flex min-w-0 items-center gap-2">
                        <Link
                          to={`/admin/plugins/${p.id}`}
                          className="min-w-0 truncate text-primary hover:underline font-medium"
                        >
                          {p.name || p.id}
                        </Link>
                        {p.frontend && (
                          <Badge
                            variant="secondary"
                            className="hidden shrink-0 gap-1 px-1.5 py-0.5 text-[10px] sm:inline-flex"
                          >
                            <Globe className="h-3 w-3" />
                            Веб-интерфейс
                          </Badge>
                        )}
                      </div>
                      {p.frontend && (
                        <a
                          href={p.frontend.url}
                          target="_blank"
                          rel="noreferrer"
                          className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-primary md:hidden"
                        >
                          <ExternalLink className="h-3 w-3" />
                          Открыть веб-интерфейс
                        </a>
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="text-muted-foreground hidden sm:table-cell">
                    {p.version || '-'}
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={p.type === 'wasm' ? 'secondary' : 'default'}
                      className={
                        p.type === 'wasm'
                          ? 'bg-purple-100 text-purple-700 hover:bg-purple-100'
                          : 'bg-blue-100 text-blue-700 hover:bg-blue-100'
                      }
                    >
                      {p.type}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <PluginStatusBadge status={p.status} />
                  </TableCell>
                  <TableCell className="hidden md:table-cell">
                    {p.frontend ? (
                      <Button variant="outline" size="sm" asChild>
                        <a href={p.frontend.url} target="_blank" rel="noreferrer">
                          <ExternalLink className="mr-1.5 h-4 w-4" />
                          Открыть
                        </a>
                      </Button>
                    ) : (
                      <span className="text-sm text-muted-foreground">-</span>
                    )}
                  </TableCell>
                  <TableCell className="text-right text-muted-foreground hidden sm:table-cell">
                    {p.triggers}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}
    </div>
  )
}
