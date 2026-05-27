import { useEffect, useState, useCallback, useContext } from 'react'
import { useParams, Link, useBeforeUnload, UNSAFE_NavigationContext as NavigationContext } from 'react-router-dom'
import { api, CommandSetting, PluginDetail } from '@/api/client'
import { toast } from 'sonner'
import RuleBuilder from '@/components/RuleBuilder'
import { Card, CardContent } from '@/components/ui/card'
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from '@/components/ui/collapsible'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import {
  AlertDialog,
  AlertDialogTrigger,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
} from '@/components/ui/alert-dialog'
import { ChevronRight, ArrowLeft, Shield } from 'lucide-react'
import { cn } from '@/lib/utils'
import { HelpTooltip } from '@/components/AdminHelp'

interface CommandRow {
  name: string
  type: string
  description: string
  enabled: boolean
  allowUserKeys: boolean
  allowServiceKeys: boolean
  policyExpression: string
  allowedOrigins: string[]
  hasSetting: boolean
}

type BlockerTx = {
  retry: () => void
}

type HistoryNavigator = {
  block: (blocker: (tx: BlockerTx) => void) => () => void
}

function useUnsavedChangesPrompt(when: boolean, message: string) {
  const navigationContext = useContext(NavigationContext)

  useBeforeUnload(
    useCallback((event) => {
      if (!when) return
      event.preventDefault()
      event.returnValue = ''
    }, [when]),
  )

  useEffect(() => {
    if (!when) return

    const navigator = navigationContext.navigator as Partial<HistoryNavigator>
    if (typeof navigator.block !== 'function') return

    const unblock = navigator.block((tx) => {
      const confirmed = window.confirm(message)
      if (!confirmed) return
      unblock()
      tx.retry()
    })

    return unblock
  }, [message, navigationContext, when])
}

function parseOrigins(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean)
}

export default function PluginCommandPermissions() {
  const { id } = useParams<{ id: string }>()
  const [plugin, setPlugin] = useState<PluginDetail | null>(null)
  const [rows, setRows] = useState<CommandRow[]>([])
  const [loading, setLoading] = useState(true)
  const [dirtyRows, setDirtyRows] = useState<string[]>([])
  const [pluginOrigins, setPluginOrigins] = useState<string[]>([])
  const [pluginOriginsText, setPluginOriginsText] = useState('')
  const [savingPluginOrigins, setSavingPluginOrigins] = useState(false)

  const loadData = useCallback(async () => {
    if (!id) return
    try {
      const p = await api.getPlugin(id)
      setPlugin(p)

      let settings: CommandSetting[] = []
      try {
        settings = await api.listCommandSettings(id)
      } catch {}
      try {
        const origins = await api.getPluginFrontendOrigins(id)
        setPluginOrigins(origins.allowed_origins ?? [])
        setPluginOriginsText((origins.allowed_origins ?? []).join('\n'))
      } catch {
        setPluginOrigins([])
        setPluginOriginsText('')
      }

      const settingMap = new Map(settings.map((s) => [s.command_name, s]))
      const commands = p.meta?.triggers?.filter((t) => t.type !== 'cron') ?? p.commands ?? []

      setRows(
        commands.map((cmd) => {
          const setting = settingMap.get(cmd.name)
          return {
            name: cmd.name,
            type: ('type' in cmd ? cmd.type : 'messenger') as string,
            description: cmd.description ?? '',
            enabled: setting?.enabled ?? true,
            allowUserKeys: setting?.allow_user_keys ?? true,
            allowServiceKeys: setting?.allow_service_keys ?? false,
            policyExpression: setting?.policy_expression ?? '',
            allowedOrigins: setting?.allowed_origins ?? [],
            hasSetting: !!setting,
          }
        }),
      )
    } catch {
      toast.error('Не удалось загрузить плагин')
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => { loadData() }, [loadData])

  const pluginOriginsDirty = pluginOriginsText !== pluginOrigins.join('\n')
  const hasUnsavedChanges = dirtyRows.length > 0 || pluginOriginsDirty
  useUnsavedChangesPrompt(hasUnsavedChanges, 'Есть несохранённые изменения. Уйти со страницы без сохранения?')

  const handleDirtyChange = useCallback((rowName: string, dirty: boolean) => {
    setDirtyRows((current) => {
      if (dirty) {
        return current.includes(rowName) ? current : [...current, rowName]
      }
      return current.filter((name) => name !== rowName)
    })
  }, [])

  const enabledCount = rows.filter((r) => r.enabled).length
  const hasHTTPTriggers = rows.some((r) => r.type === 'http')

  const handleSavePluginOrigins = async () => {
    if (!id || !pluginOriginsDirty) return
    setSavingPluginOrigins(true)
    try {
      const origins = parseOrigins(pluginOriginsText)
      await api.setPluginFrontendOrigins(id, origins)
      setPluginOrigins(origins)
      setPluginOriginsText(origins.join('\n'))
      toast.success('Frontend origins сохранены')
    } catch {
      toast.error('Не удалось сохранить frontend origins')
    } finally {
      setSavingPluginOrigins(false)
    }
  }

  if (loading) {
    return (
      <div>
        <div className="mb-6">
          <Skeleton className="h-4 w-32 mb-2" />
          <Skeleton className="h-8 w-64 mb-1" />
          <Skeleton className="h-4 w-80" />
        </div>
        <div className="space-y-4">
          {[1, 2, 3].map((i) => (
            <Card key={i}>
              <div className="flex items-center justify-between p-5">
                <div className="flex items-center gap-3">
                  <Skeleton className="h-4 w-4" />
                  <Skeleton className="h-4 w-28" />
                  <Skeleton className="h-4 w-48" />
                </div>
                <Skeleton className="h-5 w-10 rounded-full" />
              </div>
            </Card>
          ))}
        </div>
      </div>
    )
  }

  if (!plugin) return <div className="text-destructive text-sm">Плагин не найден</div>

  return (
    <div>
      <div className="mb-6">
        <Button variant="link" asChild className="p-0 h-auto text-sm">
          <Link to={`/admin/plugins/${id}`}>
            <ArrowLeft className="mr-1 h-3.5 w-3.5" />
            Назад к {plugin.name || id}
          </Link>
        </Button>
        <div className="flex flex-wrap items-start justify-between gap-3 mt-2">
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-2xl font-semibold">Права доступа к триггерам</h1>
              <HelpTooltip>
                Здесь настраивается доступность точек запуска: включены ли они,
                кто может вызывать HTTP-адреса и какая политика доступа применяется.
              </HelpTooltip>
              {rows.length > 0 && (
                <Badge variant="secondary" className="font-normal">
                  {enabledCount} из {rows.length} включены
                </Badge>
              )}
            </div>
            <p className="text-sm text-muted-foreground mt-1">
              Управление доступом к триггерам <strong>{plugin.name || id}</strong>.
            </p>
            {hasUnsavedChanges && (
              <p className="text-sm text-amber-700 mt-2">
                Есть несохранённые изменения. Сохраните их перед уходом со страницы.
              </p>
            )}
          </div>
          <div className="flex flex-wrap justify-end gap-2">
            {hasHTTPTriggers && (
              <Button variant="outline" asChild>
                <Link to="/admin/http/service-keys">
                  Сервисные ключи
                </Link>
              </Button>
            )}
          </div>
        </div>
      </div>

      {rows.length === 0 ? (
        <Card className="p-10">
          <div className="flex flex-col items-center justify-center text-center">
            <div className="rounded-full bg-muted p-4 mb-4">
              <Shield className="h-8 w-8 text-muted-foreground" />
            </div>
            <p className="text-sm font-medium text-muted-foreground mb-1">
              Нет триггеров
            </p>
            <p className="text-xs text-muted-foreground">
              У этого плагина нет настраиваемых триггеров
            </p>
          </div>
        </Card>
      ) : (
        <div className="space-y-4">
          {hasHTTPTriggers && (
            <Card>
              <CardContent className="p-5">
                <div className="mb-3 flex items-center justify-between gap-3">
                  <div>
                    <div className="flex items-center gap-2">
                      <h2 className="text-base font-medium">
                        {plugin?.frontend ? 'Внешние frontend origins' : 'Frontend origins плагина'}
                      </h2>
                      <HelpTooltip>
                        {plugin?.frontend
                          ? 'Встроенный веб-интерфейс работает на том же origin, поэтому этот список нужен только для отдельных внешних frontend-приложений.'
                          : 'Эти origins применяются ко всем HTTP-триггерам плагина. У конкретного HTTP-триггера можно задать override ниже.'}
                      </HelpTooltip>
                    </div>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {plugin?.frontend
                        ? 'Same-origin bundle не требует CORS. Добавляйте сюда только внешние origins, например http://127.0.0.1:5173'
                        : 'По одному origin на строку, например http://127.0.0.1:5173'}
                    </p>
                  </div>
                  {pluginOriginsDirty && <Badge variant="outline">не сохранено</Badge>}
                </div>
                <Textarea
                  value={pluginOriginsText}
                  disabled={savingPluginOrigins}
                  onChange={(event) => setPluginOriginsText(event.target.value)}
                  placeholder="http://127.0.0.1:5173"
                  className="min-h-[88px] font-mono text-xs"
                />
                <div className="mt-3 flex justify-end gap-2">
                  {pluginOriginsDirty && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPluginOriginsText(pluginOrigins.join('\n'))}
                      disabled={savingPluginOrigins}
                    >
                      Отменить изменения
                    </Button>
                  )}
                  <Button
                    size="sm"
                    onClick={handleSavePluginOrigins}
                    disabled={savingPluginOrigins || !pluginOriginsDirty}
                  >
                    {savingPluginOrigins ? 'Сохранение...' : 'Сохранить'}
                  </Button>
                </div>
              </CardContent>
            </Card>
          )}
          {rows.map((row) => (
            <CommandCard
              key={row.name}
              pluginId={id!}
              row={row}
              onUpdate={loadData}
              onDirtyChange={handleDirtyChange}
            />
          ))}
        </div>
      )}
    </div>
  )
}

function CommandCard({
  pluginId,
  row,
  onUpdate,
  onDirtyChange,
}: {
  pluginId: string
  row: CommandRow
  onUpdate: () => void
  onDirtyChange: (rowName: string, dirty: boolean) => void
}) {
  const [expanded, setExpanded] = useState(false)
  const [toggling, setToggling] = useState(false)
  const [savingChanges, setSavingChanges] = useState(false)
  const [allowUserKeys, setAllowUserKeys] = useState(row.allowUserKeys)
  const [allowServiceKeys, setAllowServiceKeys] = useState(row.allowServiceKeys)
  const [policyExpr, setPolicyExpr] = useState(row.policyExpression)
  const [originsText, setOriginsText] = useState(row.allowedOrigins.join('\n'))
  const [builderKey, setBuilderKey] = useState(0)
  const [clearOpen, setClearOpen] = useState(false)
  const policyVisible = row.type !== 'http' || allowUserKeys
  const savedOriginsText = row.allowedOrigins.join('\n')
  const isDirty =
    allowUserKeys !== row.allowUserKeys ||
    allowServiceKeys !== row.allowServiceKeys ||
    policyExpr !== row.policyExpression ||
    originsText !== savedOriginsText
  const isPublicHTTPTrigger =
    row.type === 'http' &&
    !allowUserKeys &&
    !allowServiceKeys &&
    policyExpr.trim() === ''

  useEffect(() => {
    setAllowUserKeys(row.allowUserKeys)
    setAllowServiceKeys(row.allowServiceKeys)
    setPolicyExpr(row.policyExpression)
    setOriginsText(row.allowedOrigins.join('\n'))
  }, [row.allowUserKeys, row.allowServiceKeys, row.policyExpression, row.allowedOrigins])

  useEffect(() => {
    onDirtyChange(row.name, isDirty)
    return () => onDirtyChange(row.name, false)
  }, [isDirty, onDirtyChange, row.name])

  const handleToggle = async () => {
    setToggling(true)
    try {
      await api.setCommandEnabled(pluginId, row.name, !row.enabled)
      onUpdate()
    } catch {
      toast.error('Не удалось переключить команду')
    } finally {
      setToggling(false)
    }
  }

  const handleSaveChanges = async () => {
    if (!isDirty) return
    setSavingChanges(true)
    try {
      if (row.type === 'http') {
        await api.setCommandAccess(pluginId, row.name, {
          allow_user_keys: allowUserKeys,
          allow_service_keys: allowServiceKeys,
        })
        await api.setCommandOrigins(pluginId, row.name, parseOrigins(originsText))
      }
      await api.setCommandPolicy(pluginId, row.name, policyExpr)
      onUpdate()
      toast.success('Изменения сохранены')
    } catch {
      toast.error('Не удалось сохранить изменения')
    } finally {
      setSavingChanges(false)
    }
  }

  const resetChanges = () => {
    setAllowUserKeys(row.allowUserKeys)
    setAllowServiceKeys(row.allowServiceKeys)
    setPolicyExpr(row.policyExpression)
    setOriginsText(row.allowedOrigins.join('\n'))
    setBuilderKey((k) => k + 1)
  }

  return (
    <Card>
      <Collapsible open={expanded} onOpenChange={setExpanded}>
        <div className="flex items-center justify-between p-5">
          <CollapsibleTrigger asChild>
            <button className="flex items-center gap-3 cursor-pointer hover:opacity-80 transition-opacity">
              <ChevronRight
                className={cn(
                  'h-4 w-4 text-muted-foreground transition-transform',
                  expanded && 'rotate-90',
                )}
              />
              <span
                className={cn(
                  'inline-block h-2 w-2 rounded-full shrink-0',
                  row.enabled ? 'bg-green-500' : 'bg-red-500',
                )}
              />
              <span className="font-mono text-sm font-medium">
                {row.type === 'messenger' ? `/${row.name}` : row.name}
              </span>
              {row.type !== 'messenger' && (
                <span className="inline-flex items-center rounded-md border px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider bg-muted text-muted-foreground">
                  {row.type}
                </span>
              )}
              {row.description && (
                <span className="text-sm text-muted-foreground">{row.description}</span>
              )}
            </button>
          </CollapsibleTrigger>
          <div className="flex items-center gap-3">
            {isPublicHTTPTrigger && (
              <Badge variant="secondary">публичный</Badge>
            )}
            {policyExpr && (
              <Badge variant="secondary">политика</Badge>
            )}
            {row.type === 'http' && allowUserKeys && (
              <Badge variant="secondary">сессия</Badge>
            )}
            {row.type === 'http' && allowServiceKeys && (
              <Badge variant="secondary">ключ</Badge>
            )}
            {row.type === 'http' && row.allowedOrigins.length > 0 && (
              <Badge variant="secondary">override origins: {row.allowedOrigins.length}</Badge>
            )}
            {isDirty && (
              <Badge variant="outline">не сохранено</Badge>
            )}
            <Switch
              checked={row.enabled}
              onCheckedChange={handleToggle}
              disabled={toggling}
            />
          </div>
        </div>

        <CollapsibleContent>
          <Separator />
          <CardContent
            className={cn(
              'p-5',
              !row.enabled && 'opacity-50 pointer-events-none',
            )}
          >
            {row.type === 'http' && (
              <>
                <div className="mb-4">
                  <div className="mb-3 flex items-center gap-2">
                    <h4 className="text-sm font-medium">Кто может вызывать HTTP-точку</h4>
                    <HelpTooltip>
                      Для HTTP-точки можно отдельно разрешить вызовы по пользовательской
                      сессии и по сервисным ключам внешних систем.
                    </HelpTooltip>
                  </div>
                  <div className="space-y-3">
                    <div className="flex items-center justify-between rounded-lg border p-3">
                      <div>
                        <div className="text-sm font-medium">Пользовательская сессия</div>
                        <div className="text-xs text-muted-foreground">
                          Доступ для пользователя, который уже вошёл в систему
                        </div>
                      </div>
                      <Switch
                        checked={allowUserKeys}
                        disabled={savingChanges}
                        onCheckedChange={setAllowUserKeys}
                      />
                    </div>
                    <div className="flex items-center justify-between rounded-lg border p-3">
                      <div>
                        <div className="text-sm font-medium">Сервисный ключ</div>
                        <div className="text-xs text-muted-foreground">
                          Доступ для внешней системы с ключом, которому разрешена эта HTTP-точка
                        </div>
                        <Button variant="link" size="sm" asChild className="h-auto px-0 mt-1">
                          <Link to="/admin/http/service-keys">
                            Управление ключами
                          </Link>
                        </Button>
                      </div>
                      <Switch
                        checked={allowServiceKeys}
                        disabled={savingChanges}
                        onCheckedChange={setAllowServiceKeys}
                      />
                    </div>
                  </div>
                </div>

                <Separator className="my-3" />

                <div className="mb-4">
                  <div className="mb-3 flex items-center gap-2">
                    <h4 className="text-sm font-medium">Override frontend origins</h4>
                    <HelpTooltip>
                      Если список пустой, HTTP-точка наследует frontend origins плагина.
                      Заполните его только когда этой точке нужен отдельный список origins.
                    </HelpTooltip>
                  </div>
                  <Textarea
                    value={originsText}
                    disabled={savingChanges}
                    onChange={(event) => setOriginsText(event.target.value)}
                    placeholder="https://schedule.example.com"
                    className="min-h-[88px] font-mono text-xs"
                  />
                </div>

                <Separator className="my-3" />
              </>
            )}

            {policyVisible && (
              <>
                <div className="flex items-center justify-between mb-3">
                  <h4 className="text-sm font-medium">
                    Политика доступа
                    <HelpTooltip className="ml-1 h-6 w-6 align-middle">
                      Выражение проверяется перед запуском точки. Пустая строка означает,
                      что дополнительная политика не применяется.
                    </HelpTooltip>
                    {policyExpr
                      ? <span className="ml-2 text-xs text-primary font-normal">(активна)</span>
                      : <span className="ml-2 text-xs text-muted-foreground font-normal">{row.type === 'http' ? '(пусто = без дополнительных ограничений для пользовательской сессии)' : '(пусто = доступно всем)'}</span>
                    }
                  </h4>
                </div>

                <RuleBuilder key={builderKey} expression={policyExpr} onChange={setPolicyExpr} />

                <Separator className="my-3" />
              </>
            )}
            <div className="flex justify-end gap-2">
              {policyVisible && policyExpr && (
                <AlertDialog open={clearOpen} onOpenChange={setClearOpen}>
                  <AlertDialogTrigger asChild>
                    <Button variant="outline" size="sm">
                      Очистить
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>Очистить политику?</AlertDialogTitle>
                      <AlertDialogDescription>
                        Политика доступа будет удалена. Точка запуска станет доступна всем пользователям.
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>Отмена</AlertDialogCancel>
                      <Button
                        size="sm"
                        onClick={() => {
                          setPolicyExpr('')
                          setBuilderKey((k) => k + 1)
                          setClearOpen(false)
                        }}
                      >
                        Очистить
                      </Button>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              )}
              {isDirty && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={resetChanges}
                  disabled={savingChanges}
                >
                  Отменить изменения
                </Button>
              )}
              <Button
                size="sm"
                onClick={handleSaveChanges}
                disabled={savingChanges || !isDirty}
              >
                {savingChanges ? 'Сохранение...' : 'Сохранить'}
              </Button>
            </div>
          </CardContent>
        </CollapsibleContent>
      </Collapsible>
    </Card>
  )
}
