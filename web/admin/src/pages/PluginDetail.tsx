import { useEffect, useState, useCallback } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { api, PluginDetail as PluginDetailType, PluginUpdatePreviewResponse } from '@/api/client'
import {
  ArrowLeft,
  Settings,
  Shield,
  History,
  Upload,
  Trash2,
  Power,
  Lock,
  Copy,
  Package,
  Clock,
  Globe,
  Zap,
  MessageSquare,
  ExternalLink,
} from 'lucide-react'
import { toast } from 'sonner'
import PluginStatusBadge from '@/components/PluginStatusBadge'
import PluginUpdatePreview from '@/components/PluginUpdatePreview'
import WasmUploader from '@/components/WasmUploader'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import {
  AlertDialog,
  AlertDialogTrigger,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogAction,
  AlertDialogCancel,
} from '@/components/ui/alert-dialog'
import {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { formatDate, getErrorMessage } from '@/lib/utils'
import { HelpTooltip } from '@/components/AdminHelp'

function describeCron(expr: string): string {
  const parts = expr.trim().split(/\s+/)
  if (parts.length < 5) return expr

  const [min, hour, dom, mon, dow] = parts

  if (min === '*' && hour === '*') return 'каждую минуту'
  if (hour === '*' && min !== '*') return `каждый час в :${min.padStart(2, '0')}`
  if (dom === '*' && mon === '*' && dow === '*' && min !== '*' && hour !== '*')
    return `каждый день в ${hour}:${min.padStart(2, '0')}`
  if (dow !== '*' && dom === '*' && mon === '*') {
    const days: Record<string, string> = {
      '0': 'вс', '1': 'пн', '2': 'вт', '3': 'ср',
      '4': 'чт', '5': 'пт', '6': 'сб', '7': 'вс',
    }
    const dayList = dow.split(',').map((d) => days[d] || d).join(', ')
    return `${dayList} в ${hour}:${min.padStart(2, '0')}`
  }

  if (min.startsWith('*/')) return `каждые ${min.slice(2)} мин`
  if (hour.startsWith('*/')) return `каждые ${hour.slice(2)} ч`

  return expr
}

const triggerIcon: Record<string, typeof Clock> = {
  messenger: MessageSquare,
  cron: Clock,
  http: Globe,
  event: Zap,
}

const triggerLabel: Record<string, string> = {
  messenger: 'Команда',
  cron: 'Расписание',
  http: 'HTTP',
  event: 'Событие',
}

function TriggerBadge({ type }: { type: string }) {
  const colors: Record<string, string> = {
    messenger: 'bg-violet-500/10 text-violet-600 border-violet-500/20',
    cron: 'bg-blue-500/10 text-blue-600 border-blue-500/20',
    http: 'bg-emerald-500/10 text-emerald-600 border-emerald-500/20',
    event: 'bg-amber-500/10 text-amber-600 border-amber-500/20',
  }
  return (
    <span className={`inline-flex items-center rounded-md border px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider ${colors[type] || 'bg-muted text-muted-foreground'}`}>
      {triggerLabel[type] || type}
    </span>
  )
}

function LoadingSkeleton() {
  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0 space-y-2">
          <Skeleton className="h-8 w-20" />
          <Skeleton className="h-6 w-48" />
          <Skeleton className="h-4 w-32" />
        </div>
        <Skeleton className="h-6 w-24 rounded-full" />
      </div>

      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-4 w-48" />
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="space-y-1.5">
                <Skeleton className="h-3 w-16" />
                <Skeleton className="h-5 w-24" />
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-24" />
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-3">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-8 w-28 rounded-md" />
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

export default function PluginDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [plugin, setPlugin] = useState<PluginDetailType | null>(null)
  const [loading, setLoading] = useState(true)
  const [showUpdate, setShowUpdate] = useState(false)
  const [showUpdatePreview, setShowUpdatePreview] = useState(false)
  const [actionLoading, setActionLoading] = useState(false)
  const [updatePreview, setUpdatePreview] = useState<PluginUpdatePreviewResponse | null>(null)
  const [updateFile, setUpdateFile] = useState<File | null>(null)
  const [updateChangelog, setUpdateChangelog] = useState('')

  const load = useCallback(() => {
    if (!id) return
    setLoading(true)
    api
      .getPlugin(id)
      .then(setPlugin)
      .catch((e: Error) => toast.error(e.message))
      .finally(() => setLoading(false))
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  const handleToggle = async () => {
    if (!id || !plugin) return
    const wasActive = plugin.status === 'active'
    setPlugin((prev) =>
      prev ? { ...prev, status: wasActive ? 'disabled' : 'active' } : prev,
    )
    try {
      if (wasActive) {
        await api.disablePlugin(id)
        toast.success('Плагин отключён')
      } else {
        await api.enablePlugin(id)
        toast.success('Плагин включён')
      }
      load()
    } catch (e: unknown) {
      setPlugin((prev) =>
        prev ? { ...prev, status: wasActive ? 'active' : 'disabled' } : prev,
      )
      toast.error(getErrorMessage(e))
    }
  }

  const handleDelete = async () => {
    if (!id) return
    setActionLoading(true)
    try {
      await api.deletePlugin(id)
      toast.success('Плагин удалён')
      navigate('/admin/plugins')
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setActionLoading(false)
    }
  }

  const resetUpdateSelection = () => {
    setUpdatePreview(null)
    setUpdateFile(null)
    setUpdateChangelog('')
  }

  const handleUpdateFile = async (file: File) => {
    if (!id) return
    setActionLoading(true)
    try {
      const preview = await api.previewPluginUpdate(id, file)
      setUpdateFile(file)
      setUpdatePreview(preview)
      setShowUpdate(false)
      setShowUpdatePreview(true)
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setActionLoading(false)
    }
  }

  const doUpdate = async (file: File) => {
    if (!id) return
    setActionLoading(true)
    try {
      await api.updatePlugin(id, file, updateChangelog)
      toast.success('Плагин обновлён')
      setShowUpdate(false)
      setShowUpdatePreview(false)
      resetUpdateSelection()
      load()
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setActionLoading(false)
    }
  }

  const handleCopyHash = () => {
    if (!plugin?.wasm_hash) return
    navigator.clipboard.writeText(plugin.wasm_hash).then(
      () => toast.success('Hash скопирован'),
      () => toast.error('Не удалось скопировать'),
    )
  }

  const handleCopyFrontendURL = () => {
    if (!plugin?.frontend?.url) return
    navigator.clipboard.writeText(plugin.frontend.url).then(
      () => toast.success('URL веб-интерфейса скопирован'),
      () => toast.error('Не удалось скопировать URL'),
    )
  }

  if (loading && !plugin) {
    return <LoadingSkeleton />
  }

  if (!plugin) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <Package className="h-12 w-12 text-muted-foreground/50 mb-4" />
        <h3 className="text-lg font-semibold mb-1">Плагин не найден</h3>
        <p className="text-sm text-muted-foreground mb-4">
          Плагин не существует или был удалён
        </p>
        <Button variant="outline" asChild>
          <Link to="/admin/plugins">
            <ArrowLeft className="mr-1.5 h-4 w-4" />
            Вернуться к списку
          </Link>
        </Button>
      </div>
    )
  }

  const allTriggers = plugin.meta?.triggers ?? []

  const statusBorderColor =
    plugin.status === 'active'
      ? 'border-l-green-500'
      : plugin.status === 'error'
        ? 'border-l-red-500'
        : 'border-l-muted-foreground/40'

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <Button variant="ghost" size="sm" asChild className="mb-2 -ml-2">
            <Link to="/admin/plugins">
              <ArrowLeft className="mr-1 h-4 w-4" />
              Назад
            </Link>
          </Button>
          <div className="flex min-w-0 items-center gap-2">
            <h2 className="truncate text-lg font-semibold">
              {plugin.name || plugin.id}
            </h2>
          </div>
          <p className="text-sm text-muted-foreground">
            {plugin.id}
            {plugin.version && <span> &middot; v{plugin.version}</span>}
          </p>
        </div>
        <PluginStatusBadge status={plugin.status || 'disabled'} />
      </div>

      {/* Plugin info card */}
      <Card className={`border-l-4 ${statusBorderColor}`}>
        <CardHeader>
          <CardTitle className="text-base">Информация</CardTitle>
          <CardDescription>Основные параметры плагина</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
            <div>
              <span className="text-muted-foreground block text-xs uppercase tracking-wide">
                Тип
              </span>
              <div className="font-medium mt-0.5">{plugin.type || 'wasm'}</div>
            </div>
            <div>
              <span className="text-muted-foreground block text-xs uppercase tracking-wide">
                Версия
              </span>
              <div className="font-medium mt-0.5">
                {plugin.version || '-'}
              </div>
            </div>
            {plugin.wasm_hash && (
              <div className="col-span-2 md:col-span-1">
                <span className="text-muted-foreground block text-xs uppercase tracking-wide">
                  Hash
                </span>
                <div className="flex items-center gap-1.5 mt-0.5">
                  <span
                    className="font-mono text-xs truncate"
                    title={plugin.wasm_hash}
                  >
                    {plugin.wasm_hash}
                  </span>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-5 w-5 shrink-0"
                    onClick={handleCopyHash}
                  >
                    <Copy className="h-3 w-3" />
                  </Button>
                </div>
              </div>
            )}
            {plugin.installed_at && (
              <div>
                <span className="text-muted-foreground block text-xs uppercase tracking-wide">
                  Установлен
                </span>
                <div className="font-medium mt-0.5">
                  {formatDate(plugin.installed_at)}
                </div>
              </div>
            )}
            {plugin.updated_at && (
              <div>
                <span className="text-muted-foreground block text-xs uppercase tracking-wide">
                  Обновлён
                </span>
                <div className="font-medium mt-0.5">
                  {formatDate(plugin.updated_at)}
                </div>
              </div>
            )}
          </div>

          {/* Triggers */}
          {allTriggers.length > 0 && (
            <>
              <Separator />
              <div>
                <div className="mb-2 flex items-center gap-2">
                  <h4 className="text-sm font-medium">
                    Триггеры ({allTriggers.length})
                  </h4>
                  <HelpTooltip>
                    Точка запуска - способ вызвать плагин: команда в мессенджере,
                    HTTP-адрес, расписание или событие.
                  </HelpTooltip>
                </div>
                <div className="space-y-1">
                  {allTriggers.map((t) => {
                    const Icon = triggerIcon[t.type] || Zap
                    return (
                      <div
                        key={`${t.type}-${t.name}`}
                        className="flex items-center gap-3 text-sm p-2 bg-muted/50 rounded-md"
                      >
                        <Icon className="h-4 w-4 text-muted-foreground shrink-0" />
                        <span className={`shrink-0 ${t.type === 'messenger' ? 'font-mono text-primary' : 'font-medium'}`}>
                          {t.type === 'messenger' ? `/${t.name}` : t.name}
                        </span>
                        {t.description && (
                          <span className="text-muted-foreground min-w-0 truncate">
                            {t.description}
                          </span>
                        )}
                        <div className="ml-auto flex items-center gap-2 shrink-0">
                          {t.type === 'messenger' && t.min_role && (
                            <Badge variant="outline">
                              {t.min_role}
                            </Badge>
                          )}
                          {t.type === 'cron' && t.schedule && (
                            <span
                              className="font-mono text-xs text-muted-foreground"
                              title={t.schedule}
                            >
                              {describeCron(t.schedule)}
                            </span>
                          )}
                          {t.type === 'http' && t.path && (
                            <span className="font-mono text-xs text-muted-foreground">
                              {t.methods?.join(', ') || 'GET'} {t.path}
                            </span>
                          )}
                          {t.type === 'event' && t.topic && (
                            <span className="font-mono text-xs text-muted-foreground">
                              {t.topic}
                            </span>
                          )}
                          <TriggerBadge type={t.type} />
                        </div>
                      </div>
                    )
                  })}
                </div>
              </div>
            </>
          )}

          {/* Permissions (legacy display from DB) */}
          {plugin.permissions && plugin.permissions.length > 0 && (
            <>
              <Separator />
              <div>
                <h4 className="text-sm font-medium mb-2">Активные доступы</h4>
                <div className="flex flex-wrap gap-2">
                  {plugin.permissions.map((p) => (
                    <Badge key={p} variant="secondary" className="font-mono">
                      {p}
                    </Badge>
                  ))}
                </div>
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {plugin.frontend && (
        <Card>
          <CardHeader>
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div className="min-w-0">
                <div className="mb-1 flex flex-wrap items-center gap-2">
                  <CardTitle className="text-base">Веб-интерфейс</CardTitle>
                  <Badge variant="secondary" className="gap-1">
                    <Globe className="h-3 w-3" />
                    Встроен в bundle
                  </Badge>
                </div>
                <CardDescription>
                  Страница плагина раздаётся Core и открывается по админской сессии.
                </CardDescription>
              </div>
              <Button size="sm" className="w-full sm:w-auto" asChild>
                <a href={plugin.frontend.url} target="_blank" rel="noreferrer">
                  <ExternalLink className="mr-1.5 h-4 w-4" />
                  Открыть
                </a>
              </Button>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-4 text-sm sm:grid-cols-3">
              <div className="sm:col-span-2">
                <span className="text-muted-foreground block text-xs uppercase tracking-wide">
                  URL
                </span>
                <div className="mt-1 flex min-w-0 items-center gap-2">
                  <span className="min-w-0 truncate font-mono text-xs" title={plugin.frontend.url}>
                    {plugin.frontend.url}
                  </span>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7 shrink-0"
                    onClick={handleCopyFrontendURL}
                  >
                    <Copy className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>
              <div>
                <span className="text-muted-foreground block text-xs uppercase tracking-wide">
                  Entrypoint
                </span>
                <div className="mt-1 font-mono text-xs">
                  {plugin.frontend.entrypoint}
                </div>
              </div>
              <div>
                <span className="text-muted-foreground block text-xs uppercase tracking-wide">
                  Assets
                </span>
                <div className="mt-1 font-medium">
                  {plugin.frontend.assets}
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Actions */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <CardTitle className="text-base">Действия</CardTitle>
            <HelpTooltip>
              Управление ниже влияет на включение плагина, обновление, удаление,
              права точек запуска и значения параметров без пересборки файла.
            </HelpTooltip>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Navigation group */}
          <div>
            <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
              Навигация
            </span>
            <div className="flex flex-wrap gap-3 mt-2">
              <Button variant="outline" size="sm" asChild>
                <Link to={`/admin/plugins/${id}/config`}>
                  <Settings className="mr-1.5 h-4 w-4" />
                  Настроить
                </Link>
              </Button>

              <Button variant="outline" size="sm" asChild>
                <Link to={`/admin/plugins/${id}/permissions`}>
                  <Shield className="mr-1.5 h-4 w-4" />
                  Права триггеров
                </Link>
              </Button>

              <Button variant="outline" size="sm" asChild>
                <Link to={`/admin/plugins/${id}/plugin-permissions`}>
                  <Lock className="mr-1.5 h-4 w-4" />
                  Требования
                </Link>
              </Button>

              <Button variant="outline" size="sm" asChild>
                <Link to={`/admin/plugins/${id}/versions`}>
                  <History className="mr-1.5 h-4 w-4" />
                  Версии
                </Link>
              </Button>

            </div>
          </div>

          <Separator />

          {/* Management group */}
          <div>
            <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
              Управление
            </span>
            <div className="flex flex-wrap gap-3 mt-2">
              <Button
                variant="outline"
                size="sm"
                onClick={handleToggle}
                disabled={actionLoading}
              >
                <Power className="mr-1.5 h-4 w-4" />
                {plugin.status === 'active' ? 'Отключить' : 'Включить'}
              </Button>

              {/* Update module dialog */}
              <Dialog
                open={showUpdate}
                onOpenChange={(open) => {
                  if (actionLoading) return
                  setShowUpdate(open)
                }}
              >
                <DialogTrigger asChild>
                  <Button variant="outline" size="sm">
                    <Upload className="mr-1.5 h-4 w-4" />
                    Обновить
                  </Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>Загрузить новый модуль</DialogTitle>
                    <DialogDescription>
                      Выберите .wasm или .zip bundle для обновления плагина{' '}
                      <strong>{plugin.name || plugin.id}</strong>. Перед применением
                      покажем сравнение изменений.
                    </DialogDescription>
                  </DialogHeader>
                  <WasmUploader onFile={handleUpdateFile} loading={actionLoading} />
                </DialogContent>
              </Dialog>

              {updatePreview && (
                <PluginUpdatePreview
                  open={showUpdatePreview}
                  loading={actionLoading}
                  preview={updatePreview}
                  changelog={updateChangelog}
                  onChangelogChange={setUpdateChangelog}
                  onCancel={() => {
                    setShowUpdatePreview(false)
                    resetUpdateSelection()
                  }}
                  onConfirm={() => {
                    if (updateFile) doUpdate(updateFile)
                  }}
                />
              )}

              {/* Delete alert dialog */}
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button variant="destructive" size="sm">
                    <Trash2 className="mr-1.5 h-4 w-4" />
                    Удалить
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>Удалить плагин</AlertDialogTitle>
                    <AlertDialogDescription>
                      Вы уверены, что хотите удалить{' '}
                      <strong>{plugin.name || plugin.id}</strong>? Это действие
                      нельзя отменить.
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel disabled={actionLoading}>
                      Отмена
                    </AlertDialogCancel>
                    <AlertDialogAction
                      onClick={handleDelete}
                      disabled={actionLoading}
                      className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                    >
                      {actionLoading ? 'Удаление...' : 'Подтвердить удаление'}
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
