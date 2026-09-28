import { useEffect, useState } from 'react'
import { Pencil, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { api, RefItem } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import CascadingSelect, { CascadeLevel } from './CascadingSelect'

const levels: CascadeLevel[] = [
  { key: 'faculty', label: 'Факультет', fetchFn: () => api.listFaculties() },
  { key: 'department', label: 'Кафедра', fetchFn: id => api.listDepartments(id!) },
  { key: 'program', label: 'Направление', fetchFn: id => api.listPrograms(id!) },
  { key: 'stream', label: 'Поток', fetchFn: id => api.listStreams(id!) },
  { key: 'group', label: 'Группа', fetchFn: id => api.listGroups(id!) },
]
const teacherLevels = levels.slice(0, 2)

export default function PositionActions({ personId, positionId, kind, name, onChanged }: {
  personId: number; positionId: number; kind: 'student' | 'teacher'; name: string; onChanged: () => void
}) {
  const [mode, setMode] = useState<'edit' | 'delete' | null>(null)
  const [busy, setBusy] = useState(false)
  const [cascade, setCascade] = useState<Record<string, number | undefined>>({})
  const [fields, setFields] = useState<Record<string, string>>({})
  const [subgroups, setSubgroups] = useState<RefItem[]>([])
  const [selected, setSelected] = useState<number[]>([])
  const [subgroupsLoading, setSubgroupsLoading] = useState(false)
  const groupId = cascade.group
  useEffect(() => {
    let active = true
    setSubgroups([])
    if (kind !== 'student' || !groupId || mode !== 'edit') return
    setSubgroupsLoading(true)
    api.listSubgroups(groupId).then(rows => { if (active) setSubgroups(rows) })
      .catch(e => { if (active) toast.error(e.message) })
      .finally(() => { if (active) setSubgroupsLoading(false) })
    return () => { active = false }
  }, [groupId, kind, mode])
  async function edit() {
    setBusy(true)
    try {
      const all = await api.getPersonPositions(personId)
      if (kind === 'student') {
        const p = all.student.find(p => p.id === positionId)
        if (!p) throw new Error('Позиция уже удалена. Обновите список.')
        setCascade({ faculty: p.faculty_id, department: p.department_id, program: p.program_id, stream: p.stream_id, group: p.study_group_id })
        setFields({ status: p.status, nationality_type: p.nationality_type, funding_type: p.funding_type, education_form: p.education_form })
        setSelected(p.subgroups?.map(s => s.id) || [])
      } else {
        const p = all.teacher.find(p => p.id === positionId)
        if (!p) throw new Error('Позиция уже удалена. Обновите список.')
        setCascade({ faculty: p.faculty_id, department: p.department_id })
        setFields({ status: p.status, position_title: p.position_title, employment_type: p.employment_type })
      }
      setMode('edit')
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Не удалось загрузить позицию') }
    finally { setBusy(false) }
  }
  async function submit() {
    if (busy) return
    if (mode === 'edit' && (kind === 'student' ? !cascade.group : !cascade.department || !fields.position_title?.trim())) {
      toast.error(kind === 'student' ? 'Выберите учебную группу' : 'Укажите кафедру и должность'); return
    }
    setBusy(true)
    try {
      if (mode === 'delete') await api.deletePersonPosition(personId, kind, positionId)
      else await api.updatePersonPosition(personId, kind, positionId, kind === 'student' ? {
        ...fields, program_id: cascade.program, stream_id: cascade.stream, study_group_id: cascade.group, subgroup_ids: selected,
      } : { ...fields, department_id: cascade.department })
      toast.success(mode === 'delete' ? 'Позиция удалена' : 'Позиция обновлена')
      setMode(null)
      onChanged()
    } catch (e) { toast.error(e instanceof Error ? e.message : 'Не удалось изменить позицию') }
    finally { setBusy(false) }
  }
  const enums: [string, string, Record<string, string>][] = [
    ['status', 'Статус', { active: 'Активен', suspended: 'Приостановлен', ended: 'Завершён' }],
    ...(kind === 'teacher' ? [
      ['employment_type', 'Занятость', { full_time: 'Штатный', part_time: 'Совместитель', hourly: 'Почасовик' }],
    ] : [
      ['nationality_type', 'Гражданство', { domestic: 'РФ', foreign: 'Иностранный' }],
      ['funding_type', 'Финансирование', { budget: 'Бюджет', contract: 'Контракт' }],
      ['education_form', 'Форма обучения', { full_time: 'Очная', part_time: 'Заочная', remote: 'Дистант' }],
    ]) as [string, string, Record<string, string>][],
  ]
  return <>
    <div className="flex justify-end gap-1">
      <Button variant="ghost" size="icon" title="Редактировать позицию" aria-label={`Редактировать позицию: ${name}`} disabled={busy} onClick={edit}><Pencil className="h-4 w-4" /></Button>
      <Button variant="ghost" size="icon" title="Удалить позицию" aria-label={`Удалить позицию: ${name}`} disabled={busy} onClick={() => setMode('delete')}><Trash2 className="h-4 w-4 text-destructive" /></Button>
    </div>
    <Dialog open={mode !== null} onOpenChange={open => { if (!open && !busy) setMode(null) }}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader><DialogTitle>{mode === 'delete' ? 'Удалить позицию?' : kind === 'student' ? 'Редактировать студенческую позицию' : 'Редактировать позицию преподавателя'}</DialogTitle></DialogHeader>
        <p className="text-sm">{name}</p>
        {mode === 'delete' ? <p className="text-sm text-muted-foreground">Будет удалена только эта позиция. Персона, пользователь бота и остальные позиции сохранятся.</p> : <fieldset disabled={busy} className="space-y-4">
          <CascadingSelect levels={kind === 'student' ? levels : teacherLevels} values={cascade} onChange={next => { if (next.group !== cascade.group) setSelected([]); setCascade(next) }} />
          {kind === 'teacher' && <div className="space-y-2"><Label>Должность</Label><Input maxLength={100} value={fields.position_title || ''} onChange={e => setFields({ ...fields, position_title: e.target.value })} /></div>}
          <div className="grid grid-cols-2 gap-3">{enums.map(([key, label, options]) => <div className="space-y-2" key={key}>
            <Label>{label}</Label><Select disabled={busy} value={fields[key]} onValueChange={value => setFields({ ...fields, [key]: value })}>
              <SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{Object.entries(options).map(([value, text]) => <SelectItem key={value} value={value}>{text}</SelectItem>)}</SelectContent>
            </Select>
          </div>)}</div>
          {kind === 'student' && subgroups.length > 0 && <div className="space-y-2"><Label>Подгруппы</Label>{subgroups.map(s => <label key={s.id} className="flex gap-2 text-sm">
            <input type="checkbox" checked={selected.includes(s.id)} onChange={() => setSelected(ids => ids.includes(s.id) ? ids.filter(id => id !== s.id) : [...ids, s.id])} />{s.name}
          </label>)}</div>}
        </fieldset>}
        <DialogFooter><Button variant="outline" disabled={busy} onClick={() => setMode(null)}>Отмена</Button><Button variant={mode === 'delete' ? 'destructive' : 'default'} disabled={busy || (mode === 'edit' && kind === 'student' && subgroupsLoading)} onClick={submit}>{busy ? 'Сохранение...' : mode === 'delete' ? 'Удалить' : 'Сохранить'}</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  </>
}
