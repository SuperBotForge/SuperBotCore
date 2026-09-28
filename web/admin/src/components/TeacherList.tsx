import PositionActions from './PositionActions'
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, ImportedTeacherInfo, ManualTeacherCreateRequest, RefItem } from '@/api/client'
import { BadgeCheck, Plus, UserRoundSearch } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { toast } from 'sonner'

const initialForm = (): ManualTeacherCreateRequest => ({
  external_id: '', last_name: '', first_name: '', middle_name: '', email: '', phone: '',
  department_id: undefined, position_title: '', employment_type: 'full_time', status: 'active',
})

type TeacherListProps = {
  items: ImportedTeacherInfo[]
  loading: boolean
  onCreated: () => void
}

export default function TeacherList({ items, loading, onCreated }: TeacherListProps) {
  const [open, setOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [form, setForm] = useState(initialForm)
  const [faculties, setFaculties] = useState<RefItem[]>([])
  const [faculty, setFaculty] = useState('')
  const [departments, setDepartments] = useState<RefItem[]>([])
  useEffect(() => {
    if (open) api.listFaculties().then(setFaculties).catch(e => toast.error(e.message))
  }, [open])
  useEffect(() => {
    let active = true
    setDepartments([])
    if (faculty) api.listDepartments(Number(faculty)).then(data => { if (active) setDepartments(data) })
      .catch(e => { if (active) toast.error(e.message) })
    return () => { active = false }
  }, [faculty])
  async function save(e: React.FormEvent) {
    e.preventDefault()
    if (!form.department_id) { toast.error('Выберите кафедру'); return }
    setSaving(true)
    try {
      await api.createImportedTeacher(form)
      setOpen(false)
      onCreated()
      toast.success('Позиция преподавателя добавлена')
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Не удалось добавить преподавателя') }
    finally { setSaving(false) }
  }
  return <Card>
    <CardHeader className="flex flex-row items-center justify-between">
      <CardTitle className="text-base">Преподаватели</CardTitle>
      <Button size="sm" onClick={() => { setForm(initialForm()); setFaculty(''); setOpen(true) }}><Plus className="mr-1.5 h-4 w-4" />Добавить преподавателя</Button>
    </CardHeader>
    <CardContent>
      <Table>
        <TableHeader><TableRow>
          {['External ID', 'ФИО', 'Контакты', 'Кафедра', 'Должность', 'Статус', 'Связь с ботом'].map(h => <TableHead key={h}>{h}</TableHead>)}
          <TableHead className="text-right">Действия</TableHead>
        </TableRow></TableHeader>
        <TableBody>{loading && items.length === 0 ? (
          Array.from({ length: 5 }).map((_, index) => <TableRow key={index}>
            {[24, 40, 36, 28, 28, 16, 20, 16].map((width, column) => <TableCell key={column}><div className="h-4 animate-pulse rounded bg-muted" style={{ width: `${width / 4}rem` }} /></TableCell>)}
          </TableRow>)
        ) : items.length === 0 ? (
          <TableRow><TableCell colSpan={8}><div className="flex flex-col items-center py-10 text-center">
            <UserRoundSearch className="mb-3 h-10 w-10 text-muted-foreground/40" />
            <p className="text-sm font-medium">Преподаватели не найдены</p>
          </div></TableCell></TableRow>
        ) : items.map(p => <TableRow key={p.position_id}>
          <TableCell className="font-mono text-sm">{p.external_id || '-'}</TableCell>
          <TableCell>{[p.last_name, p.first_name, p.middle_name].filter(Boolean).join(' ')}</TableCell>
          <TableCell><div className="text-sm"><div>{p.email || '-'}</div><div className="text-muted-foreground">{p.phone || '-'}</div></div></TableCell>
          <TableCell>{p.department_name || '-'}</TableCell>
          <TableCell>{p.position_title}</TableCell><TableCell><Badge variant={p.status === 'active' ? 'default' : 'secondary'}>{p.status}</Badge></TableCell>
          <TableCell>{p.global_user_id ? (
            <Button variant="outline" size="sm" asChild>
              <Link to={`/admin/users/${p.global_user_id}`}><BadgeCheck className="mr-1.5 h-4 w-4" />User #{p.global_user_id}</Link>
            </Button>
          ) : <Badge variant="outline">Не привязан</Badge>}</TableCell>
        <TableCell><PositionActions personId={p.id} positionId={p.position_id} kind="teacher" name={[p.last_name, p.first_name, p.middle_name].filter(Boolean).join(' ')} onChanged={onCreated} /></TableCell>
        </TableRow>)}</TableBody>
      </Table>
    </CardContent>
    <Dialog open={open} onOpenChange={v => { if (!saving) setOpen(v) }}>
      <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
        <DialogHeader><DialogTitle>Добавить преподавателя</DialogTitle></DialogHeader>
        <form onSubmit={save} className="space-y-4">
          <p className="text-sm text-muted-foreground">Для автоматической привязки укажите AccountId ТГУ в поле External ID. Если персона уже существует, используйте её External ID и ФИО.</p>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            {([
              ['external_id', 'External ID', true], ['last_name', 'Фамилия', true], ['first_name', 'Имя', true],
              ['middle_name', 'Отчество', false], ['email', 'Email', false], ['phone', 'Телефон', false],
            ] as const).map(([key, label, required]) => <div key={key} className="space-y-2">
              <Label htmlFor={`teacher-${key}`}>{label}{required ? ' *' : ''}</Label>
              <Input id={`teacher-${key}`} required={required} type={key === 'email' ? 'email' : 'text'} value={form[key] || ''} disabled={saving}
                onChange={e => setForm({ ...form, [key]: e.target.value })} />
            </div>)}
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="space-y-2"><Label>Факультет</Label>
              <Select value={faculty} disabled={saving} onValueChange={v => { setFaculty(v); setForm({ ...form, department_id: undefined }) }}>
                <SelectTrigger><SelectValue placeholder="Выберите факультет" /></SelectTrigger>
                <SelectContent>{faculties.map(f => <SelectItem key={f.id} value={String(f.id)}>{f.name}</SelectItem>)}</SelectContent>
              </Select>
            </div>
            <div className="space-y-2"><Label>Кафедра *</Label>
              <Select value={form.department_id ? String(form.department_id) : ''} disabled={!faculty || saving} onValueChange={v => setForm({ ...form, department_id: Number(v) })}>
                <SelectTrigger><SelectValue placeholder="Выберите кафедру" /></SelectTrigger>
                <SelectContent>{departments.map(d => <SelectItem key={d.id} value={String(d.id)}>{d.name}</SelectItem>)}</SelectContent>
              </Select>
            </div>
          </div>
          <div className="space-y-2"><Label htmlFor="teacher-title">Должность *</Label>
            <Input id="teacher-title" required maxLength={100} disabled={saving} placeholder="Например, старший преподаватель" value={form.position_title} onChange={e => setForm({ ...form, position_title: e.target.value })} />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2"><Label>Занятость</Label>
              <Select value={form.employment_type} disabled={saving} onValueChange={v => setForm({ ...form, employment_type: v })}>
                <SelectTrigger><SelectValue /></SelectTrigger><SelectContent>
                  <SelectItem value="full_time">Основное место работы</SelectItem><SelectItem value="part_time">Совместительство</SelectItem><SelectItem value="hourly">Почасовая</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2"><Label>Статус</Label>
              <Select value={form.status} disabled={saving} onValueChange={v => setForm({ ...form, status: v })}>
                <SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{['active', 'suspended', 'ended'].map(s => <SelectItem key={s} value={s}>{s}</SelectItem>)}</SelectContent>
              </Select>
            </div>
          </div>
          <div className="flex justify-end gap-2"><Button type="button" variant="outline" disabled={saving} onClick={() => setOpen(false)}>Отмена</Button><Button type="submit" disabled={saving}>{saving ? 'Сохранение...' : 'Добавить преподавателя'}</Button></div>
        </form>
      </DialogContent>
    </Dialog>
  </Card>
}
