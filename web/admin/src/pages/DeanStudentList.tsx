import { useEffect, useState, useCallback } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { deanApi, StudentRow, GroupBrief } from '@/api/client'
import { toast } from 'sonner'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { Search, X } from 'lucide-react'

const STATUS_LABELS: Record<string, string> = {
  active: 'Активен',
  suspended: 'Академотпуск',
  ended: 'Завершил',
}

const FUNDING_LABELS: Record<string, string> = {
  budget: 'Бюджет',
  contract: 'Контракт',
}

const NATIONALITY_LABELS: Record<string, string> = {
  domestic: 'Отечественный',
  foreign: 'Иностранец',
}

const EDUCATION_FORM_LABELS: Record<string, string> = {
  full_time: 'Очная',
  part_time: 'Заочная',
  remote: 'Дистанционная',
}

function StudentCard({ s }: { s: StudentRow }) {
  const fullName = [s.last_name, s.first_name, s.middle_name].filter(Boolean).join(' ')
  return (
    <Link
      to={`/dean/students/${s.position_id}`}
      className="flex items-center gap-4 rounded-lg border p-4 hover:bg-muted/50 transition-colors"
    >
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="font-medium text-sm">{fullName}</span>
          {s.external_id && (
            <span className="text-xs text-muted-foreground font-mono">{s.external_id}</span>
          )}
        </div>
        <div className="flex items-center gap-2 mt-1 flex-wrap">
          {s.group_code && (
            <span className="text-xs text-muted-foreground">Группа: {s.group_code}</span>
          )}
          {s.program_name && (
            <span className="text-xs text-muted-foreground">— {s.program_name}</span>
          )}
        </div>
        {(s.email || s.phone) && (
          <div className="text-xs text-muted-foreground mt-0.5">
            {s.email}{s.email && s.phone ? ' · ' : ''}{s.phone}
          </div>
        )}
      </div>
      <div className="flex flex-col items-end gap-1 shrink-0">
        <Badge variant={s.status === 'active' ? 'default' : 'secondary'}>
          {STATUS_LABELS[s.status] ?? s.status}
        </Badge>
        <div className="flex gap-1">
          <Badge variant="outline" className="text-xs">{FUNDING_LABELS[s.funding_type] ?? s.funding_type}</Badge>
          {s.nationality_type === 'foreign' && (
            <Badge variant="outline" className="text-xs">Иностранец</Badge>
          )}
        </div>
      </div>
    </Link>
  )
}

export default function DeanStudentList() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [students, setStudents] = useState<StudentRow[]>([])
  const [groups, setGroups] = useState<GroupBrief[]>([])
  const [loading, setLoading] = useState(true)

  const groupId = searchParams.get('group_id') ?? ''
  const status = searchParams.get('status') ?? ''
  const fundingType = searchParams.get('funding_type') ?? ''
  const nationalityType = searchParams.get('nationality_type') ?? ''
  const [searchText, setSearchText] = useState(searchParams.get('search') ?? '')

  const loadStudents = useCallback(() => {
    setLoading(true)
    deanApi.listStudents({
      group_id: groupId ? Number(groupId) : undefined,
      status: status || undefined,
      funding_type: fundingType || undefined,
      nationality_type: nationalityType || undefined,
      search: searchText || undefined,
    })
      .then(setStudents)
      .catch((e: Error) => toast.error(e.message))
      .finally(() => setLoading(false))
  }, [groupId, status, fundingType, nationalityType, searchText])

  useEffect(() => {
    deanApi.listGroups().then(setGroups).catch(() => {})
  }, [])

  useEffect(() => { loadStudents() }, [loadStudents])

  const setParam = (key: string, value: string) => {
    const p = new URLSearchParams(searchParams)
    if (value) p.set(key, value)
    else p.delete(key)
    setSearchParams(p)
  }

  const clearFilters = () => {
    setSearchText('')
    setSearchParams({})
  }

  const hasFilters = groupId || status || fundingType || nationalityType || searchText

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Студенты</h1>
        {hasFilters && (
          <Button variant="ghost" size="sm" onClick={clearFilters}>
            <X className="mr-1 h-3.5 w-3.5" />
            Сбросить фильтры
          </Button>
        )}
      </div>

      <div className="flex flex-wrap gap-3">
        <div className="relative flex-1 min-w-48">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Поиск по имени, email, логину..."
            className="pl-8"
            value={searchText}
            onChange={e => setSearchText(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') loadStudents() }}
          />
        </div>

        <Select value={groupId} onValueChange={v => setParam('group_id', v === '__all__' ? '' : v)}>
          <SelectTrigger className="w-40">
            <SelectValue placeholder="Все группы" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">Все группы</SelectItem>
            {groups.map(g => (
              <SelectItem key={g.id} value={String(g.id)}>{g.code}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={status} onValueChange={v => setParam('status', v === '__all__' ? '' : v)}>
          <SelectTrigger className="w-44">
            <SelectValue placeholder="Все статусы" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">Все статусы</SelectItem>
            <SelectItem value="active">Активен</SelectItem>
            <SelectItem value="suspended">Академотпуск</SelectItem>
            <SelectItem value="ended">Завершил</SelectItem>
          </SelectContent>
        </Select>

        <Select value={fundingType} onValueChange={v => setParam('funding_type', v === '__all__' ? '' : v)}>
          <SelectTrigger className="w-40">
            <SelectValue placeholder="Финансирование" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">Все</SelectItem>
            <SelectItem value="budget">Бюджет</SelectItem>
            <SelectItem value="contract">Контракт</SelectItem>
          </SelectContent>
        </Select>

        <Select value={nationalityType} onValueChange={v => setParam('nationality_type', v === '__all__' ? '' : v)}>
          <SelectTrigger className="w-44">
            <SelectValue placeholder="Гражданство" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">Все</SelectItem>
            <SelectItem value="domestic">Отечественные</SelectItem>
            <SelectItem value="foreign">Иностранцы</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {loading ? (
        <div className="space-y-2">
          {[1,2,3,4,5].map(i => <Skeleton key={i} className="h-20" />)}
        </div>
      ) : students.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground">
            Студенты не найдены
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-2">
          <p className="text-sm text-muted-foreground">{students.length} студентов</p>
          {students.map(s => <StudentCard key={s.position_id} s={s} />)}
        </div>
      )}
    </div>
  )
}

export { STATUS_LABELS, FUNDING_LABELS, NATIONALITY_LABELS, EDUCATION_FORM_LABELS }
