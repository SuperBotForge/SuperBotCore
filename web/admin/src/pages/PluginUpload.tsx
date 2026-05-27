import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ArrowLeft, Clock, Globe, Radio, Terminal, TriangleAlert } from 'lucide-react'
import { api, PluginMeta } from '@/api/client'
import WasmUploader from '@/components/WasmUploader'
import RequirementsPanel from '@/components/RequirementsPanel'
import JsonSchemaForm, { validateSchema } from '@/components/JsonSchemaForm'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from '@/components/ui/card'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogAction,
  AlertDialogCancel,
} from '@/components/ui/alert-dialog'
import { cn, compareVersions, getErrorMessage } from '@/lib/utils'
import { HelpTooltip } from '@/components/AdminHelp'

const steps = [
  { num: 1, label: 'Загрузка файла' },
  { num: 2, label: 'Проверка метаданных' },
  { num: 3, label: 'Установка' },
]

function StepIndicator({ current }: { current: number }) {
  return (
    <div className="flex items-center justify-center mb-8">
      {steps.map((step, i) => (
        <div key={step.num} className="flex items-center">
          <div className="flex flex-col items-center">
            <div
              className={cn(
                'flex items-center justify-center w-8 h-8 rounded-full border-2 text-sm font-semibold transition-colors',
                current >= step.num
                  ? 'border-primary bg-primary text-primary-foreground'
                  : 'border-muted-foreground/30 text-muted-foreground',
              )}
            >
              {step.num}
            </div>
            <span
              className={cn(
                'text-xs mt-1.5 whitespace-nowrap',
                current >= step.num ? 'text-primary font-medium' : 'text-muted-foreground',
              )}
            >
              {step.label}
            </span>
          </div>
          {i < steps.length - 1 && (
            <div
              className={cn(
                'w-16 sm:w-24 h-0.5 mx-2 mb-5 transition-colors',
                current > step.num ? 'bg-primary' : 'bg-muted-foreground/20',
              )}
            />
          )}
        </div>
      ))}
    </div>
  )
}

export default function PluginUpload() {
  const navigate = useNavigate()
  const [uploading, setUploading] = useState(false)
  const [installing, setInstalling] = useState(false)
  const [meta, setMeta] = useState<PluginMeta | null>(null)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [configValues, setConfigValues] = useState<Record<string, unknown>>({})
  const [configErrors, setConfigErrors] = useState<Record<string, string>>({})

  const currentStep = installing ? 3 : meta ? 2 : 1

  const versionConflict = meta?.existing_version
    ? compareVersions(meta.version, meta.existing_version)
    : null

  const triggers = meta?.triggers ?? []

  const handleFile = async (file: File) => {
    setUploading(true)
    try {
      const result = await api.uploadPlugin(file)
      result.triggers = result.triggers ?? []
      result.requirements = result.requirements ?? []
      setMeta(result)
      toast.success('Файл загружен, проверьте метаданные ниже')
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setUploading(false)
    }
  }

  const hasConfigSchema = meta?.config_schema && Object.keys(meta.config_schema).length > 0

  const doInstall = async () => {
    if (!meta) return
    if (hasConfigSchema) {
      const errs = validateSchema(meta.config_schema as Record<string, unknown>, configValues)
      if (Object.keys(errs).length > 0) {
        setConfigErrors(errs)
        toast.error('Заполните обязательные поля конфигурации')
        return
      }
    }
    setInstalling(true)
    try {
      await api.installPlugin(meta.id, {
        wasm_key: meta.wasm_key,
        config: hasConfigSchema ? configValues : {},
      })
      toast.success('Плагин успешно установлен')
      navigate(`/admin/plugins/${meta.id}`)
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setInstalling(false)
    }
  }

  const handleInstall = () => {
    if (meta?.existing_version) {
      setConfirmOpen(true)
    } else {
      doInstall()
    }
  }

  const handleReset = () => {
    setMeta(null)
    setConfigValues({})
    setConfigErrors({})
  }

  return (
    <div>
      <div className="mb-6">
        <Button variant="ghost" size="sm" asChild className="mb-2 -ml-2">
          <Link to="/admin/plugins">
            <ArrowLeft className="mr-1 h-4 w-4" />
            Назад
          </Link>
        </Button>
        <div className="flex flex-wrap items-center gap-2">
          <h2 className="text-lg font-semibold">Загрузка плагина</h2>
          <HelpTooltip>
            После выбора файла админка покажет, что будет установлено: название, версию,
            точки запуска, ресурсы и обязательные настройки. Установка начнётся только после
            кнопки «Установить».
          </HelpTooltip>
        </div>
        <p className="text-sm text-muted-foreground mt-1">
          Загрузите .wasm файл или .zip bundle с frontend/
        </p>
      </div>

      <StepIndicator current={currentStep} />

      {!meta && <WasmUploader onFile={handleFile} loading={uploading} />}

      {meta && (
        <Card>
          <CardHeader>
            <CardTitle className="text-2xl font-bold">{meta.name}</CardTitle>
            <CardDescription className="text-sm">
              {meta.id} &middot; v{meta.version}
            </CardDescription>
            {meta.wasm_hash && (
              <Badge variant="secondary" className="w-fit font-mono text-xs truncate max-w-full">
                SHA: {meta.wasm_hash}
              </Badge>
            )}
          </CardHeader>

          <CardContent className="space-y-6">
            {meta.existing_version && (
              <Card className="border-amber-200 bg-amber-50/50 dark:border-amber-900 dark:bg-amber-950/30">
                <CardContent className="flex items-start gap-3 p-4">
                  <TriangleAlert className="h-5 w-5 text-amber-500 mt-0.5 shrink-0" />
                  <div className="text-sm text-amber-700 dark:text-amber-400">
                    {versionConflict === 0 ? (
                      <p>
                        Плагин <strong>{meta.id}</strong> версии <strong>v{meta.existing_version}</strong> уже установлен. Вы загружаете ту же версию.
                      </p>
                    ) : versionConflict !== null && versionConflict < 0 ? (
                      <p>
                        Установлена более новая версия <strong>v{meta.existing_version}</strong>. Вы загружаете более старую — <strong>v{meta.version}</strong>.
                      </p>
                    ) : (
                      <p>
                        Плагин <strong>{meta.id}</strong> уже установлен (v{meta.existing_version}). Будет обновлён до <strong>v{meta.version}</strong>.
                      </p>
                    )}
                  </div>
                </CardContent>
              </Card>
            )}

            {meta.frontend && (
              <div className="rounded-md border bg-muted/30 p-3 text-sm">
                <div className="mb-1 flex items-center gap-2 font-medium">
                  <Globe className="h-4 w-4 text-muted-foreground" />
                  Bundle содержит веб-интерфейс
                </div>
                <div className="text-muted-foreground">
                  После установки страница будет доступна по админской сессии:
                  <span className="ml-1 font-mono text-primary">{meta.frontend.url}</span>
                </div>
                <div className="mt-1 text-xs text-muted-foreground">
                  {meta.frontend.assets} файлов, входная точка{' '}
                  <span className="font-mono">{meta.frontend.entrypoint}</span>
                </div>
              </div>
            )}

            {triggers.length > 0 && (
              <div>
                <div className="mb-2 flex items-center gap-2">
                  <h4 className="text-sm font-medium text-muted-foreground">
                    Триггеры ({triggers.length})
                  </h4>
                  <HelpTooltip>
                    Точка запуска показывает, как именно будет запускаться плагин: из мессенджера,
                    по HTTP, по расписанию или по событию.
                  </HelpTooltip>
                </div>
                <div className="space-y-1">
                  {triggers.map((t) => (
                    <div
                      key={`${t.type}-${t.name}`}
                      className="flex items-center gap-3 text-sm p-2 bg-muted/50 rounded"
                    >
                      <span className="shrink-0 text-muted-foreground">
                        {t.type === 'messenger' && <Terminal className="h-4 w-4" />}
                        {t.type === 'http' && <Globe className="h-4 w-4" />}
                        {t.type === 'cron' && <Clock className="h-4 w-4" />}
                        {t.type === 'event' && <Radio className="h-4 w-4" />}
                      </span>
                      <span className="font-mono text-primary shrink-0">
                        {t.type === 'messenger' && `/${t.name}`}
                        {t.type === 'http' && `${(t.methods ?? ['GET']).join(',')} ${t.path}`}
                        {t.type === 'cron' && t.schedule}
                        {t.type === 'event' && t.topic}
                      </span>
                      <span className="text-muted-foreground min-w-0 truncate">
                        {t.description}
                      </span>
                      {t.min_role && (
                        <Badge variant="outline" className="ml-auto shrink-0">
                          {t.min_role}
                        </Badge>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {meta.requirements.length > 0 && (
              <RequirementsPanel requirements={meta.requirements} />
            )}

            {hasConfigSchema && (
              <div>
                <div className="mb-2 flex items-center gap-2">
                  <h4 className="text-sm font-medium text-muted-foreground">Конфигурация</h4>
                  <HelpTooltip>
                    Эти поля объявлены плагином как обязательные или доступные настройки.
                    Заполненные значения будут сохранены вместе с установкой.
                  </HelpTooltip>
                </div>
                <JsonSchemaForm
                  schema={meta.config_schema as Record<string, unknown>}
                  value={configValues}
                  onChange={setConfigValues}
                  errors={configErrors}
                />
              </div>
            )}
          </CardContent>

          <CardFooter className="gap-3">
            <Button onClick={handleInstall} disabled={installing}>
              {installing ? 'Установка...' : 'Установить'}
            </Button>
            <Button variant="outline" onClick={handleReset} disabled={installing}>
              Отмена
            </Button>
          </CardFooter>
        </Card>
      )}

      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {versionConflict === 0
                ? 'Такая версия уже установлена'
                : versionConflict !== null && versionConflict < 0
                  ? 'Откат на старую версию'
                  : 'Обновление плагина'}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {versionConflict === 0 ? (
                <>
                  Плагин <strong>{meta?.id}</strong> версии <strong>v{meta?.existing_version}</strong> уже
                  установлен. Вы уверены, что хотите переустановить ту же версию?
                </>
              ) : versionConflict !== null && versionConflict < 0 ? (
                <>
                  Сейчас установлена версия <strong>v{meta?.existing_version}</strong>, а вы устанавливаете
                  более старую — <strong>v{meta?.version}</strong>. Продолжить?
                </>
              ) : (
                <>
                  Плагин <strong>{meta?.id}</strong> будет обновлён с <strong>v{meta?.existing_version}</strong> до{' '}
                  <strong>v{meta?.version}</strong>. Продолжить?
                </>
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Отмена</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                setConfirmOpen(false)
                doInstall()
              }}
            >
              {versionConflict !== null && versionConflict < 0 ? 'Всё равно установить' : 'Продолжить'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
