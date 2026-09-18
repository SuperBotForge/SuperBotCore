import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, ImportedStudentInfo, ManualStudentCreateRequest, RefItem, StudentImportResult, UserListItem } from '@/api/client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getErrorMessage } from '@/lib/utils'
import {
  BadgeCheck,
  Download,
  FileSpreadsheet,
  Loader2,
  Plus,
  Search,
  Trash2,
  Upload,
  UserRoundSearch,
  Users,
  X,
} from 'lucide-react'
import { toast } from 'sonner'
import { HelpTooltip } from '@/components/AdminHelp'

const CHANNEL_SHORT: Record<string, string> = { TELEGRAM: 'TG', DISCORD: 'DC', VK: 'VK', MATTERMOST: 'MM' }

const CHANNEL_FILTERS = [
  { value: '', label: 'Все каналы' },
  { value: 'TELEGRAM', label: 'Telegram' },
  { value: 'DISCORD', label: 'Discord' },
  { value: 'VK', label: 'VK' },
  { value: 'MATTERMOST', label: 'Mattermost' },
]

const ROLE_FILTERS = [
  { value: '', label: 'Все роли' },
  { value: 'USER', label: 'USER' },
  { value: 'ADMIN', label: 'ADMIN' },
]

const STATUS_OPTIONS = [
  { value: 'active', label: 'active' },
  { value: 'suspended', label: 'suspended' },
  { value: 'ended', label: 'ended' },
]

const NATIONALITY_OPTIONS = [
  { value: 'domestic', label: 'domestic' },
  { value: 'foreign', label: 'foreign' },
]

const FUNDING_OPTIONS = [
  { value: 'budget', label: 'budget' },
  { value: 'contract', label: 'contract' },
]

const EDUCATION_OPTIONS = [
  { value: 'full_time', label: 'full_time' },
  { value: 'part_time', label: 'part_time' },
  { value: 'remote', label: 'remote' },
]

type ManualStudentForm = {
  external_id: string
  last_name: string
  first_name: string
  middle_name: string
  email: string
  phone: string
  faculty_id?: number
  department_id?: number
  program_id?: number
  stream_id?: number
  group_id?: number
  subgroup_ids: number[]
  status: string
  nationality_type: string
  funding_type: string
  education_form: string
}

const initialManualStudentForm = (): ManualStudentForm => ({
  external_id: '',
  last_name: '',
  first_name: '',
  middle_name: '',
  email: '',
  phone: '',
  faculty_id: undefined,
  department_id: undefined,
  program_id: undefined,
  stream_id: undefined,
  group_id: undefined,
  subgroup_ids: [],
  status: 'active',
  nationality_type: 'domestic',
  funding_type: 'budget',
  education_form: 'full_time',
})

export default function UserList() {
  const [users, setUsers] = useState<UserListItem[]>([])
  const [importedStudents, setImportedStudents] = useState<ImportedStudentInfo[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [loadingImported, setLoadingImported] = useState(true)
  const [inputValue, setInputValue] = useState('')
  const [search, setSearch] = useState('')
  const [channel, setChannel] = useState('')
  const [role, setRole] = useState('')
  const [importFile, setImportFile] = useState<File | null>(null)
  const [importing, setImporting] = useState(false)
  const [importResult, setImportResult] = useState<StudentImportResult | null>(null)
  const [manualDialogOpen, setManualDialogOpen] = useState(false)
  const [manualSaving, setManualSaving] = useState(false)
  const [manualForm, setManualForm] = useState<ManualStudentForm>(initialManualStudentForm)
  const [faculties, setFaculties] = useState<RefItem[]>([])
  const [departments, setDepartments] = useState<RefItem[]>([])
  const [programs, setPrograms] = useState<RefItem[]>([])
  const [streams, setStreams] = useState<RefItem[]>([])
  const [groups, setGroups] = useState<RefItem[]>([])
  const [subgroups, setSubgroups] = useState<RefItem[]>([])
  const debounceRef = useRef<ReturnType<typeof setTimeout>>()

  const loadUsers = useCallback(() => {
    setLoading(true)
    api.listUsers({ search, channel, role })
      .then((data) => {
        setUsers(data.users || [])
        setTotal(data.total ?? data.users?.length ?? 0)
      })
      .catch((e: Error) => toast.error(e.message))
      .finally(() => setLoading(false))
  }, [search, channel, role])

  const loadImportedStudents = useCallback(() => {
    setLoadingImported(true)
    api.listImportedStudents(search)
      .then((items) => setImportedStudents(items || []))
      .catch((e: Error) => toast.error(e.message))
      .finally(() => setLoadingImported(false))
  }, [search])

  const loadFaculties = useCallback(() => {
    api.listFaculties()
      .then((items) => setFaculties(items || []))
      .catch((e: Error) => toast.error(e.message))
  }, [])

  const handleInputChange = useCallback((value: string) => {
    setInputValue(value)
    clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => setSearch(value), 300)
  }, [])

  const clearSearch = () => {
    setInputValue('')
    setSearch('')
  }

  useEffect(() => {
    return () => clearTimeout(debounceRef.current)
  }, [])

  useEffect(() => {
    loadUsers()
  }, [loadUsers])

  useEffect(() => {
    loadImportedStudents()
  }, [loadImportedStudents])

  useEffect(() => {
    if (manualDialogOpen && faculties.length === 0) {
      loadFaculties()
    }
  }, [manualDialogOpen, faculties.length, loadFaculties])

  useEffect(() => {
    if (!manualForm.faculty_id) {
      setDepartments([])
      setPrograms([])
      setStreams([])
      setGroups([])
      setSubgroups([])
      return
    }
    api.listDepartments(manualForm.faculty_id)
      .then((items) => setDepartments(items || []))
      .catch((e: Error) => toast.error(e.message))
  }, [manualForm.faculty_id])

  useEffect(() => {
    if (!manualForm.department_id) {
      setPrograms([])
      setStreams([])
      setGroups([])
      setSubgroups([])
      return
    }
    api.listPrograms(manualForm.department_id)
      .then((items) => setPrograms(items || []))
      .catch((e: Error) => toast.error(e.message))
  }, [manualForm.department_id])

  useEffect(() => {
    if (!manualForm.program_id) {
      setStreams([])
      setGroups([])
      setSubgroups([])
      return
    }
    api.listStreams(manualForm.program_id)
      .then((items) => setStreams(items || []))
      .catch((e: Error) => toast.error(e.message))
  }, [manualForm.program_id])

  useEffect(() => {
    if (!manualForm.stream_id) {
      setGroups([])
      setSubgroups([])
      return
    }
    api.listGroups(manualForm.stream_id)
      .then((items) => setGroups(items || []))
      .catch((e: Error) => toast.error(e.message))
  }, [manualForm.stream_id])

  useEffect(() => {
    if (!manualForm.group_id) {
      setSubgroups([])
      return
    }
    api.listSubgroups(manualForm.group_id)
      .then((items) => setSubgroups(items || []))
      .catch((e: Error) => toast.error(e.message))
  }, [manualForm.group_id])

  const handleDelete = async (id: number) => {
    if (!confirm('Удалить пользователя?')) return

    try {
      await api.deleteUser(id)
      setUsers((prev) => prev.filter((u) => u.id !== id))
      setTotal((prev) => Math.max(prev - 1, 0))
      toast.success('Пользователь удалён')
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    }
  }

  const handleTemplateDownload = () => {
    window.open('/api/admin/import/template', '_blank', 'noopener,noreferrer')
  }

  const handleImport = async () => {
    if (!importFile) {
      toast.error('Выберите .xlsx файл для импорта')
      return
    }

    setImporting(true)
    try {
      const result = await api.importStudents(importFile)
      setImportResult(result)
      loadUsers()
      loadImportedStudents()

      if ((result.errors ?? []).length > 0) {
        toast.warning(`Импорт завершён с ошибками: создано ${result.created}, обновлено ${result.updated}, пропущено ${result.skipped}`)
      } else {
        toast.success(`Импорт завершён: создано ${result.created}, обновлено ${result.updated}`)
      }
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setImporting(false)
    }
  }

  const resetManualForm = () => {
    setManualForm(initialManualStudentForm())
    setDepartments([])
    setPrograms([])
    setStreams([])
    setGroups([])
    setSubgroups([])
  }

  const setCascadeValue = (field: keyof ManualStudentForm, value?: number) => {
    setManualForm((prev) => {
      const next = { ...prev, [field]: value }
      if (field === 'faculty_id') {
        next.department_id = undefined
        next.program_id = undefined
        next.stream_id = undefined
        next.group_id = undefined
        next.subgroup_ids = []
      }
      if (field === 'department_id') {
        next.program_id = undefined
        next.stream_id = undefined
        next.group_id = undefined
        next.subgroup_ids = []
      }
      if (field === 'program_id') {
        next.stream_id = undefined
        next.group_id = undefined
        next.subgroup_ids = []
      }
      if (field === 'stream_id') {
        next.group_id = undefined
        next.subgroup_ids = []
      }
      if (field === 'group_id') {
        next.subgroup_ids = []
      }
      return next
    })
  }

  const subgroupCodes = useMemo(
    () => subgroups.filter((item) => manualForm.subgroup_ids.includes(item.id)).map((item) => item.code),
    [manualForm.subgroup_ids, subgroups],
  )

  const handleManualCreate = async () => {
    if (!manualForm.external_id || !manualForm.last_name || !manualForm.first_name || !manualForm.group_id) {
      toast.error('Заполните external_id, фамилию, имя и группу')
      return
    }

    const programCode = programs.find((item) => item.id === manualForm.program_id)?.code
    const streamCode = streams.find((item) => item.id === manualForm.stream_id)?.code
    const groupCode = groups.find((item) => item.id === manualForm.group_id)?.code

    if (!groupCode) {
      toast.error('Выберите учебную группу')
      return
    }

    const payload: ManualStudentCreateRequest = {
      external_id: manualForm.external_id.trim(),
      last_name: manualForm.last_name.trim(),
      first_name: manualForm.first_name.trim(),
      middle_name: manualForm.middle_name.trim(),
      email: manualForm.email.trim(),
      phone: manualForm.phone.trim(),
      program_code: programCode,
      stream_code: streamCode,
      group_code: groupCode,
      subgroup_codes: subgroupCodes,
      status: manualForm.status,
      nationality_type: manualForm.nationality_type,
      funding_type: manualForm.funding_type,
      education_form: manualForm.education_form,
    }

    setManualSaving(true)
    try {
      const result = await api.createImportedStudent(payload)
      toast.success(result.created ? 'Студент создан' : 'Студент обновлён')
      setManualDialogOpen(false)
      resetManualForm()
      loadImportedStudents()
    } catch (e: unknown) {
      toast.error(getErrorMessage(e))
    } finally {
      setManualSaving(false)
    }
  }

  const hasFilters = Boolean(search || channel || role)
  const importErrors = importResult?.errors ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h1 className="text-2xl font-bold">Пользователи</h1>
          <HelpTooltip>
            Глобальные пользователи объединяют аккаунты из мессенджеров, роль, локаль и
            учебные данные, которые затем используются в политиках доступа.
          </HelpTooltip>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <div className="relative w-72">
          {loading || loadingImported ? (
            <Loader2 className="absolute left-2.5 top-2.5 h-4 w-4 animate-spin text-muted-foreground" />
          ) : (
            <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          )}
          <Input
            placeholder="ФИО, external_id, email или ID..."
            value={inputValue}
            onChange={(e) => handleInputChange(e.target.value)}
            className="pl-8 pr-8"
          />
          {inputValue && (
            <button
              type="button"
              onClick={clearSearch}
              className="absolute right-2 top-2.5 text-muted-foreground hover:text-foreground"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>

        <div className="flex gap-1.5">
          {CHANNEL_FILTERS.map((filter) => (
            <Button
              key={filter.value}
              variant={channel === filter.value ? 'default' : 'outline'}
              size="sm"
              onClick={() => setChannel(filter.value)}
              className="h-8 text-xs"
            >
              {filter.label}
            </Button>
          ))}
        </div>

        <div className="flex gap-1.5">
          {ROLE_FILTERS.map((filter) => (
            <Button
              key={filter.value}
              variant={role === filter.value ? 'default' : 'outline'}
              size="sm"
              onClick={() => setRole(filter.value)}
              className="h-8 text-xs"
            >
              {filter.label}
            </Button>
          ))}
        </div>

        {hasFilters && <span className="text-sm text-muted-foreground">Поиск активен</span>}
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <FileSpreadsheet className="h-4 w-4" />
            Импорт студентов из Excel
            <HelpTooltip>
              Импорт создаёт или обновляет учебные позиции студентов. Эти данные нужны для
              проверок доступа по группам, потокам и другим университетским признакам.
            </HelpTooltip>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
            <Input
              type="file"
              accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
              onChange={(e) => {
                setImportResult(null)
                setImportFile(e.target.files?.[0] ?? null)
              }}
              className="max-w-xl"
            />
            <div className="flex flex-wrap gap-2">
              <Button type="button" variant="outline" onClick={handleTemplateDownload}>
                <Download className="mr-1.5 h-4 w-4" />
                Скачать шаблон
              </Button>
              <Button type="button" onClick={handleImport} disabled={!importFile || importing}>
                {importing ? <Loader2 className="mr-1.5 h-4 w-4 animate-spin" /> : <Upload className="mr-1.5 h-4 w-4" />}
                {importing ? 'Импортируем...' : 'Импортировать'}
              </Button>
            </div>
          </div>

          {importResult && (
            <div className="space-y-3 rounded-lg border p-4">
              <div className="flex flex-wrap gap-2 text-sm">
                <Badge variant="outline">Всего: {importResult.total}</Badge>
                <Badge variant="outline">Создано: {importResult.created}</Badge>
                <Badge variant="outline">Обновлено: {importResult.updated}</Badge>
                <Badge variant={importResult.skipped > 0 ? 'destructive' : 'outline'}>
                  Пропущено: {importResult.skipped}
                </Badge>
              </div>

              {importErrors.length > 0 && (
                <div className="space-y-2">
                  <p className="text-sm font-medium">Ошибки импорта</p>
                  <div className="max-h-56 overflow-y-auto rounded-md border">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead className="w-20">Строка</TableHead>
                          <TableHead className="w-40">Поле</TableHead>
                          <TableHead>Сообщение</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {importErrors.map((error, index) => (
                          <TableRow key={`${error.row}-${error.field || 'row'}-${index}`}>
                            <TableCell>{error.row}</TableCell>
                            <TableCell>{error.field || '-'}</TableCell>
                            <TableCell>{error.message}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                </div>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      <Tabs defaultValue="users" className="space-y-4">
        <TabsList>
          <TabsTrigger value="users">Пользователи бота ({total})</TabsTrigger>
          <TabsTrigger value="imported">Импортированные студенты ({importedStudents.length})</TabsTrigger>
        </TabsList>

        <TabsContent value="users">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{!hasFilters && `Всего: ${total}`}</CardTitle>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-20">ID</TableHead>
                    <TableHead>ФИО</TableHead>
                    <TableHead>Аккаунты</TableHead>
                    <TableHead className="w-16 text-right">Действия</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading ? (
                    Array.from({ length: 5 }).map((_, index) => (
                      <TableRow key={index}>
                        <TableCell><div className="h-4 w-10 animate-pulse rounded bg-muted" /></TableCell>
                        <TableCell><div className="h-4 w-32 animate-pulse rounded bg-muted" /></TableCell>
                        <TableCell><div className="h-4 w-24 animate-pulse rounded bg-muted" /></TableCell>
                        <TableCell />
                      </TableRow>
                    ))
                  ) : users.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={4}>
                        <div className="flex flex-col items-center py-10 text-center">
                          <Users className="mb-3 h-10 w-10 text-muted-foreground/40" />
                          <p className="text-sm text-muted-foreground">Пользователи бота не найдены</p>
                        </div>
                      </TableCell>
                    </TableRow>
                  ) : (
                    users.map((user) => (
                      <TableRow key={user.id}>
                        <TableCell>
                          <Link to={`/admin/users/${user.id}`} className="font-mono text-sm text-primary hover:underline">
                            {user.id}
                          </Link>
                        </TableCell>
                        <TableCell>{user.person_name || <span className="text-muted-foreground">-</span>}</TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-1.5">
                            {(user.accounts || []).filter((account) => account.channel_type).map((account, index) => (
                              <Badge key={index} variant="outline" className="font-normal">
                                {CHANNEL_SHORT[account.channel_type] || account.channel_type}
                                {(account.username || account.channel_user_id) && (
                                  <span className="ml-1 font-medium">
                                    {account.username ? `@${account.username}` : account.channel_type === 'VK' ? `id${account.channel_user_id}` : account.channel_user_id}
                                  </span>
                                )}
                              </Badge>
                            ))}
                            {(!user.accounts || user.accounts.length === 0) && <span className="text-sm text-muted-foreground">-</span>}
                          </div>
                        </TableCell>
                        <TableCell className="text-right">
                          <Button variant="ghost" size="icon" onClick={() => handleDelete(user.id)}>
                            <Trash2 className="h-4 w-4 text-destructive" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="imported">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="text-base">Студенты из Excel и ручного добавления</CardTitle>
              <Dialog
                open={manualDialogOpen}
                onOpenChange={(open) => {
                  setManualDialogOpen(open)
                  if (!open) resetManualForm()
                }}
              >
                <DialogTrigger asChild>
                  <Button size="sm">
                    <Plus className="mr-1.5 h-4 w-4" />
                    Добавить студента
                  </Button>
                </DialogTrigger>
                <DialogContent className="max-w-3xl">
                  <DialogHeader>
                    <DialogTitle>Добавить студента вручную</DialogTitle>
                  </DialogHeader>

                  <div className="grid gap-4">
                    <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                      <Field label="External ID *">
                        <Input value={manualForm.external_id} onChange={(e) => setManualForm((prev) => ({ ...prev, external_id: e.target.value }))} />
                      </Field>
                      <Field label="Фамилия *">
                        <Input value={manualForm.last_name} onChange={(e) => setManualForm((prev) => ({ ...prev, last_name: e.target.value }))} />
                      </Field>
                      <Field label="Имя *">
                        <Input value={manualForm.first_name} onChange={(e) => setManualForm((prev) => ({ ...prev, first_name: e.target.value }))} />
                      </Field>
                    </div>

                    <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                      <Field label="Отчество">
                        <Input value={manualForm.middle_name} onChange={(e) => setManualForm((prev) => ({ ...prev, middle_name: e.target.value }))} />
                      </Field>
                      <Field label="Email">
                        <Input value={manualForm.email} onChange={(e) => setManualForm((prev) => ({ ...prev, email: e.target.value }))} />
                      </Field>
                      <Field label="Телефон">
                        <Input value={manualForm.phone} onChange={(e) => setManualForm((prev) => ({ ...prev, phone: e.target.value }))} />
                      </Field>
                    </div>

                    <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-5">
                      <RefSelect
                        label="Факультет"
                        value={manualForm.faculty_id}
                        items={faculties}
                        onChange={(value) => setCascadeValue('faculty_id', value)}
                      />
                      <RefSelect
                        label="Кафедра"
                        value={manualForm.department_id}
                        items={departments}
                        disabled={!manualForm.faculty_id}
                        onChange={(value) => setCascadeValue('department_id', value)}
                      />
                      <RefSelect
                        label="Программа"
                        value={manualForm.program_id}
                        items={programs}
                        disabled={!manualForm.department_id}
                        onChange={(value) => setCascadeValue('program_id', value)}
                      />
                      <RefSelect
                        label="Поток"
                        value={manualForm.stream_id}
                        items={streams}
                        disabled={!manualForm.program_id}
                        onChange={(value) => setCascadeValue('stream_id', value)}
                      />
                      <RefSelect
                        label="Группа *"
                        value={manualForm.group_id}
                        items={groups}
                        disabled={!manualForm.stream_id}
                        onChange={(value) => setCascadeValue('group_id', value)}
                      />
                    </div>

                    <div className="grid grid-cols-1 gap-3 md:grid-cols-4">
                      <EnumSelect label="Статус" value={manualForm.status} options={STATUS_OPTIONS} onChange={(value) => setManualForm((prev) => ({ ...prev, status: value }))} />
                      <EnumSelect label="Гражданство" value={manualForm.nationality_type} options={NATIONALITY_OPTIONS} onChange={(value) => setManualForm((prev) => ({ ...prev, nationality_type: value }))} />
                      <EnumSelect label="Финансирование" value={manualForm.funding_type} options={FUNDING_OPTIONS} onChange={(value) => setManualForm((prev) => ({ ...prev, funding_type: value }))} />
                      <EnumSelect label="Форма обучения" value={manualForm.education_form} options={EDUCATION_OPTIONS} onChange={(value) => setManualForm((prev) => ({ ...prev, education_form: value }))} />
                    </div>

                    <Field label="Подгруппы">
                      <div className="max-h-36 space-y-1 overflow-y-auto rounded-md border p-2">
                        {subgroups.length === 0 ? (
                          <p className="text-sm text-muted-foreground">Сначала выберите группу. Подгруппы можно не заполнять.</p>
                        ) : (
                          subgroups.map((subgroup) => (
                            <label key={subgroup.id} className="flex items-center gap-2 rounded px-1 py-1 text-sm hover:bg-muted/50">
                              <input
                                type="checkbox"
                                checked={manualForm.subgroup_ids.includes(subgroup.id)}
                                onChange={() => {
                                  setManualForm((prev) => ({
                                    ...prev,
                                    subgroup_ids: prev.subgroup_ids.includes(subgroup.id)
                                      ? prev.subgroup_ids.filter((id) => id !== subgroup.id)
                                      : [...prev.subgroup_ids, subgroup.id],
                                  }))
                                }}
                              />
                              <span>{subgroup.name}</span>
                              <span className="text-muted-foreground">({subgroup.code})</span>
                            </label>
                          ))
                        )}
                      </div>
                    </Field>
                  </div>

                  <DialogFooter>
                    <Button variant="outline" onClick={() => { setManualDialogOpen(false); resetManualForm() }}>
                      Отмена
                    </Button>
                    <Button onClick={handleManualCreate} disabled={manualSaving}>
                      {manualSaving ? 'Сохраняем...' : 'Создать студента'}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>External ID</TableHead>
                    <TableHead>ФИО</TableHead>
                    <TableHead>Контакты</TableHead>
                    <TableHead>Направление</TableHead>
                    <TableHead>Поток / группа</TableHead>
                    <TableHead>Статус</TableHead>
                    <TableHead>Связь с ботом</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loadingImported ? (
                    Array.from({ length: 5 }).map((_, index) => (
                      <TableRow key={index}>
                        <TableCell><div className="h-4 w-24 animate-pulse rounded bg-muted" /></TableCell>
                        <TableCell><div className="h-4 w-40 animate-pulse rounded bg-muted" /></TableCell>
                        <TableCell><div className="h-4 w-36 animate-pulse rounded bg-muted" /></TableCell>
                        <TableCell><div className="h-4 w-28 animate-pulse rounded bg-muted" /></TableCell>
                        <TableCell><div className="h-4 w-28 animate-pulse rounded bg-muted" /></TableCell>
                        <TableCell><div className="h-4 w-16 animate-pulse rounded bg-muted" /></TableCell>
                        <TableCell><div className="h-4 w-20 animate-pulse rounded bg-muted" /></TableCell>
                      </TableRow>
                    ))
                  ) : importedStudents.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7}>
                        <div className="flex flex-col items-center py-10 text-center">
                          <UserRoundSearch className="mb-3 h-10 w-10 text-muted-foreground/40" />
                          <p className="text-sm font-medium">Импортированных студентов пока нет</p>
                          <p className="mt-1 text-xs text-muted-foreground">
                            Здесь будут отображаться записи из Excel и студенты, добавленные вручную
                          </p>
                        </div>
                      </TableCell>
                    </TableRow>
                  ) : (
                    importedStudents.map((student) => (
                      <TableRow key={student.person_id}>
                        <TableCell className="font-mono text-sm">{student.external_id || '-'}</TableCell>
                        <TableCell>{student.last_name} {student.first_name} {student.middle_name || ''}</TableCell>
                        <TableCell>
                          <div className="text-sm">
                            <div>{student.email || '-'}</div>
                            <div className="text-muted-foreground">{student.phone || '-'}</div>
                          </div>
                        </TableCell>
                        <TableCell>{student.program_name || '-'}</TableCell>
                        <TableCell>
                          <div className="text-sm">
                            <div>{student.stream_name || '-'}</div>
                            <div className="text-muted-foreground">{student.study_group_name || '-'}</div>
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge variant={student.status === 'active' ? 'default' : 'secondary'}>{student.status}</Badge>
                        </TableCell>
                        <TableCell>
                          {student.global_user_id ? (
                            <Button variant="outline" size="sm" asChild>
                              <Link to={`/admin/users/${student.global_user_id}`}>
                                <BadgeCheck className="mr-1.5 h-4 w-4" />
                                User #{student.global_user_id}
                              </Link>
                            </Button>
                          ) : (
                            <Badge variant="outline">Не привязан</Badge>
                          )}
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <Label className="text-xs">{label}</Label>
      {children}
    </div>
  )
}

function RefSelect({
  label,
  value,
  items,
  disabled,
  onChange,
}: {
  label: string
  value?: number
  items: RefItem[]
  disabled?: boolean
  onChange: (value?: number) => void
}) {
  return (
    <Field label={label}>
      <Select value={value ? String(value) : ''} onValueChange={(next) => onChange(next ? Number(next) : undefined)} disabled={disabled}>
        <SelectTrigger>
          <SelectValue placeholder="Выберите" />
        </SelectTrigger>
        <SelectContent>
          {items.map((item) => (
            <SelectItem key={item.id} value={String(item.id)}>
              {item.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </Field>
  )
}

function EnumSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: string
  options: Array<{ value: string; label: string }>
  onChange: (value: string) => void
}) {
  return (
    <Field label={label}>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </Field>
  )
}
