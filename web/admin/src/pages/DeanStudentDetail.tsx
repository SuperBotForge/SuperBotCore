import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { deanApi, StudentRow, GroupBrief, UpdateStudentBody } from '@/api/client'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ArrowLeft } from 'lucide-react'
import {
  STATUS_LABELS,
  FUNDING_LABELS,
  NATIONALITY_LABELS,
  EDUCATION_FORM_LABELS,
} from './DeanStudentList'

export default function DeanStudentDetail() {
  const { positionId } = useParams<{ positionId: string }>()
  const [student, setStudent] = useState<StudentRow | null>(null)
  const [groups, setGroups] = useState<GroupBrief[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  const [form, setForm] = useState<UpdateStudentBody>({
    status: '',
    nationality_type: '',
    funding_type: '',
    education_form: '',
    study_group_id: undefined,
    email: '',
    phone: '',
  })

  useEffect(() => {
    if (!positionId) return
    Promise.all([
      deanApi.getStudent(Number(positionId)),
      deanApi.listGroups(),
    ])
      .then(([s, g]) => {
        setStudent(s)
        setGroups(g)
        setForm({
          status: s.status,
          nationality_type: s.nationality_type,
          funding_type: s.funding_type,
          education_form: s.education_form,
          study_group_id: s.group_id ?? undefined,
          email: s.email ?? '',
          phone: s.phone ?? '',
        })
      })
      .catch((e: Error) => toast.error(e.message))
      .finally(() => setLoading(false))
  }, [positionId])

  const fullName = student
    ? [student.last_name, student.first_name, student.middle_name].filter(Boolean).join(' ')
    : ''

  const isDirty = student && (
    form.status !== student.status ||
    form.nationality_type !== student.nationality_type ||
    form.funding_type !== student.funding_type ||
    form.education_form !== student.education_form ||
    form.study_group_id !== (student.group_id ?? undefined) ||
    form.email !== (student.email ?? '') ||
    form.phone !== (student.phone ?? '')
  )

  const handleSave = async () => {
    if (!positionId) return
    setSaving(true)
    try {
      await deanApi.updateStudent(Number(positionId), form)
      const updated = await deanApi.getStudent(Number(positionId))
      setStudent(updated)
      toast.success('Изменения сохранены')
    } catch (e: unknown) {
      toast.error((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-6 w-32" />
        <Skeleton className="h-48" />
        <Skeleton className="h-48" />
      </div>
    )
  }

  if (!student) {
    return <div className="text-destructive text-sm">Студент не найден</div>
  }

  return (
    <div className="space-y-6">
      <div>
        <Link
          to="/dean/students"
          className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="h-4 w-4" />
          Назад к списку
        </Link>
        <div className="mt-2 flex items-center gap-3 flex-wrap">
          <h1 className="text-2xl font-semibold">{fullName}</h1>
          <Badge variant={student.status === 'active' ? 'default' : 'secondary'}>
            {STATUS_LABELS[student.status] ?? student.status}
          </Badge>
          {student.bot_user_id && (
            <Badge variant="outline" className="text-xs">В боте</Badge>
          )}
        </div>
        {student.external_id && (
          <p className="text-sm text-muted-foreground font-mono mt-1">{student.external_id}</p>
        )}
      </div>

      <div className="grid md:grid-cols-2 gap-6">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Учёба</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-1.5">
              <Label>Группа</Label>
              <Select
                value={form.study_group_id ? String(form.study_group_id) : '__none__'}
                onValueChange={v =>
                  setForm(f => ({ ...f, study_group_id: v === '__none__' ? undefined : Number(v) }))
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder="Не назначена" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">Не назначена</SelectItem>
                  {groups.map(g => (
                    <SelectItem key={g.id} value={String(g.id)}>{g.code}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label>Статус</Label>
              <Select
                value={form.status}
                onValueChange={v => setForm(f => ({ ...f, status: v }))}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {Object.entries(STATUS_LABELS).map(([v, l]) => (
                    <SelectItem key={v} value={v}>{l}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label>Финансирование</Label>
              <Select
                value={form.funding_type}
                onValueChange={v => setForm(f => ({ ...f, funding_type: v }))}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {Object.entries(FUNDING_LABELS).map(([v, l]) => (
                    <SelectItem key={v} value={v}>{l}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label>Гражданство</Label>
              <Select
                value={form.nationality_type}
                onValueChange={v => setForm(f => ({ ...f, nationality_type: v }))}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {Object.entries(NATIONALITY_LABELS).map(([v, l]) => (
                    <SelectItem key={v} value={v}>{l}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label>Форма обучения</Label>
              <Select
                value={form.education_form}
                onValueChange={v => setForm(f => ({ ...f, education_form: v }))}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {Object.entries(EDUCATION_FORM_LABELS).map(([v, l]) => (
                    <SelectItem key={v} value={v}>{l}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Контакты</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-1.5">
              <Label>Email</Label>
              <Input
                value={form.email ?? ''}
                onChange={e => setForm(f => ({ ...f, email: e.target.value }))}
                placeholder="email@example.com"
              />
            </div>
            <div className="space-y-1.5">
              <Label>Телефон</Label>
              <Input
                value={form.phone ?? ''}
                onChange={e => setForm(f => ({ ...f, phone: e.target.value }))}
                placeholder="+7 900 000-00-00"
              />
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="flex justify-end gap-3">
        <Button
          onClick={handleSave}
          disabled={saving || !isDirty}
        >
          {saving ? 'Сохранение...' : 'Сохранить изменения'}
        </Button>
      </div>
    </div>
  )
}
