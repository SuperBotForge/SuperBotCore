import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { deanApi, DeanDashboard, GroupStats } from '@/api/client'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { Users, GraduationCap, BookOpen, Globe } from 'lucide-react'

function StatCard({ label, value, icon }: { label: string; value: number; icon: React.ReactNode }) {
  return (
    <Card>
      <CardContent className="p-5 flex items-center gap-4">
        <div className="rounded-lg bg-muted p-3 shrink-0">{icon}</div>
        <div>
          <p className="text-sm text-muted-foreground">{label}</p>
          <p className="text-2xl font-semibold">{value}</p>
        </div>
      </CardContent>
    </Card>
  )
}

function GroupRow({ g }: { g: GroupStats }) {
  return (
    <Link
      to={`/dean/students?group_id=${g.group_id}`}
      className="flex items-center gap-4 rounded-lg border p-4 hover:bg-muted/50 transition-colors"
    >
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="font-mono text-sm font-medium">{g.group_code}</span>
          {g.group_name && (
            <span className="text-xs text-muted-foreground">{g.group_name}</span>
          )}
          <span className="text-xs text-muted-foreground">— {g.program_name}</span>
        </div>
      </div>
      <div className="flex items-center gap-2 shrink-0">
        <Badge variant="secondary">{g.active_students} студ.</Badge>
        {g.foreign_students > 0 && (
          <Badge variant="outline">{g.foreign_students} ин.</Badge>
        )}
        {g.contract_students > 0 && (
          <Badge variant="outline">{g.contract_students} контр.</Badge>
        )}
      </div>
    </Link>
  )
}

export default function DeanDashboardPage() {
  const [data, setData] = useState<DeanDashboard | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    deanApi.getDashboard()
      .then(setData)
      .catch((e: Error) => toast.error(e.message))
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          {[1,2,3,4].map(i => <Skeleton key={i} className="h-24" />)}
        </div>
        <Skeleton className="h-64" />
      </div>
    )
  }

  if (!data) return null

  const { faculty: f, groups } = data

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">{f.faculty_name}</h1>
        <p className="text-sm text-muted-foreground mt-1">Обзор факультета</p>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard
          label="Студентов (активных)"
          value={f.active_students}
          icon={<Users className="h-5 w-5 text-muted-foreground" />}
        />
        <StatCard
          label="Групп"
          value={f.group_count}
          icon={<BookOpen className="h-5 w-5 text-muted-foreground" />}
        />
        <StatCard
          label="Бюджетников"
          value={f.budget_students}
          icon={<GraduationCap className="h-5 w-5 text-muted-foreground" />}
        />
        <StatCard
          label="Иностранцев"
          value={f.foreign_students}
          icon={<Globe className="h-5 w-5 text-muted-foreground" />}
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Группы факультета</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          {groups.length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-6">Нет групп</p>
          ) : (
            groups.map(g => <GroupRow key={g.group_id} g={g} />)
          )}
        </CardContent>
      </Card>
    </div>
  )
}
