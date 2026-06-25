const BASE = '/api/admin'

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  return requestWithBase<T>(BASE, url, init)
}

async function requestWithBase<T>(base: string, url: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(`${base}${url}`, {
      ...init,
      headers: {
        ...(init?.headers ?? {}),
        ...(init?.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }),
      },
    })
  } catch {
    throw new ApiError('Network error: unable to reach server', 0)
  }

  let data: unknown
  const contentType = res.headers.get('content-type') ?? ''
  if (contentType.includes('application/json')) {
    try {
      data = await res.json()
    } catch {
      throw new ApiError('Invalid JSON response from server', res.status)
    }
  } else {
    const text = await res.text()
    if (!res.ok) {
      throw new ApiError(text || `HTTP ${res.status}`, res.status)
    }
    try {
      data = JSON.parse(text)
    } catch {
      throw new ApiError(`Unexpected response format (HTTP ${res.status})`, res.status)
    }
  }

  if (!res.ok) {
    const msg =
      (data && typeof data === 'object' && 'error' in data && typeof (data as Record<string, unknown>).error === 'string')
        ? (data as Record<string, string>).error
        : `HTTP ${res.status}`
    throw new ApiError(msg, res.status)
  }

  return data as T
}

export interface UserTokenInfo {
  id: number
  public_id: string
  name: string
  active: boolean
  expires_at?: string
  last_used_at?: string
  created_at: string
  updated_at: string
}

export interface CreatedUserToken extends UserTokenInfo {
  token: string
}

export interface PluginInfo {
  id: string
  name: string
  version: string
  type: 'go' | 'wasm'
  status: 'active' | 'disabled' | 'error'
  triggers: number
  frontend?: PluginFrontendSummary
}

export interface TriggerDef {
  name: string
  type: 'cron' | 'http' | 'event' | 'messenger'
  description?: string
  min_role?: string
  schedule?: string
  path?: string
  methods?: string[]
  topic?: string
}

export interface RPCMethodDef {
  name: string
  description?: string
}

export interface RequirementDef {
  type: string
  name?: string
  description: string
  target?: string
  config?: unknown
}

export interface PluginFrontendSummary {
  url: string
  entrypoint: string
  assets: number
}

export interface PluginMeta {
  id: string
  name: string
  version: string
  triggers: TriggerDef[]
  requirements: RequirementDef[]
  rpc_methods?: RPCMethodDef[]
  config_schema: Record<string, unknown> | null
  wasm_key: string
  wasm_hash: string
  existing_version?: string
  frontend?: PluginFrontendSummary
}

export interface PluginDetail {
  id: string
  name?: string
  version?: string
  type?: string
  status?: string
  commands?: { name: string; description: string; min_role?: string }[]
  meta?: PluginMeta
  config?: unknown
  permissions?: string[]
  wasm_hash?: string
  installed_at?: string
  updated_at?: string
  frontend?: PluginFrontendSummary
}

export interface PluginUpdatePreviewInfo {
  id: string
  name: string
  version?: string
}

export interface PluginUpdatePreviewSummary {
  key: string
  title: string
  current: string
  next: string
  changed: boolean
}

export interface PluginUpdatePreviewWarning {
  code: string
  level: 'info' | 'warn' | 'error'
  title: string
  message: string
}

export interface PluginUpdatePreviewItem {
  key: string
  title: string
  detail?: string
  before?: string
  after?: string
  change: 'added' | 'removed' | 'changed'
}

export interface PluginUpdatePreviewSection {
  key: string
  title: string
  added: number
  removed: number
  changed: number
  same: number
  empty_message?: string
  items: PluginUpdatePreviewItem[]
}

export interface PluginUpdatePreviewResponse {
  can_update: boolean
  has_changes: boolean
  current: PluginUpdatePreviewInfo
  next: PluginUpdatePreviewInfo
  summary: PluginUpdatePreviewSummary[]
  warnings: PluginUpdatePreviewWarning[]
  sections: PluginUpdatePreviewSection[]
}

export interface CommandSetting {
  id: number
  plugin_id: string
  command_name: string
  enabled: boolean
  allow_user_keys: boolean
  allow_service_keys: boolean
  policy_expression: string
  allowed_origins: string[]
  created_at: string
  updated_at: string
}

export interface PluginFrontendOrigins {
  plugin_id: string
  allowed_origins: string[]
  created_at?: string
  updated_at?: string
}

export interface ServiceKeyScope {
  plugin_id: string
  trigger_name: string
}

export interface ServiceKeyInfo {
  id: number
  public_id: string
  name: string
  active: boolean
  expires_at?: string
  last_used_at?: string
  created_at: string
  updated_at: string
  scopes: ServiceKeyScope[]
}

export interface CreatedServiceKey extends ServiceKeyInfo {
  token: string
}

export interface RuleParamOption {
  value: string
  label: string
}

export interface RuleParam {
  name: string
  label: string
  type: 'select' | 'text' | 'text_or_select'
  placeholder?: string
  options?: RuleParamOption[]
  depends_on?: string
}

export interface RuleConditionType {
  id: string
  label: string
  template: string
  params: RuleParam[]
}

export interface RuleSchema {
  condition_types: RuleConditionType[]
  field_values: Record<string, RuleParamOption[]>
}

export interface RequirementInfo {
  type: string
  description: string
  target?: string
}

export interface PluginRequirementsDetail {
  requirements: RequirementInfo[]
}

export interface VersionInfo {
  id: number
  plugin_id: string
  version: string
  wasm_key: string
  wasm_hash: string
  config_json: unknown
  permissions: string[]
  changelog: string
  created_at: string
}

export interface StudentTripChecklistStep {
  key: string
  label: string
  status: string
  can_action: boolean
  action?: string
}

export interface StudentTrip {
  id: number
  user_id: number
  full_name: string
  departure_date: string
  expected_return: string
  actual_return?: string
  status: string
  stamp_status: string
  stamp_file_id?: string
  stamp_file_name?: string
  stamp_mime_type?: string
  stamp_url?: string
  stamp_uploaded_at?: string
  arrival_reported_at?: string
  rejection_reason?: string
  created_at: string
  updated_at: string
  summary: string
  checklist: StudentTripChecklistStep[]
}

export interface StudentTripsResponse {
  trips: StudentTrip[]
}

export interface StudentTripActionResponse {
  status: string
  notify_error?: string
  trip: StudentTrip
}

export interface ChannelStatus {
  name: string
  type: string
  status: 'connected' | 'disconnected' | 'not_configured'
}

export interface ChatReference {
  id: number
  channel_type: string
  platform_chat_id: string
  chat_kind: string
  title: string
}

export interface BroadcastResult {
  chat_id: number
  channel_type: string
  status: 'sent' | 'error'
  error?: string
}

export interface AccountBrief {
  channel_type: string
  username?: string
}

export interface UserListItem {
  id: number
  locale: string
  role: string
  person_name?: string
  accounts: AccountBrief[]
  created_at?: string
}

export interface UserDetail {
  id: number
  primary_channel: string
  tsu_accounts_id?: string
  locale: string
  role: string
  person_name?: string
  accounts: AccountInfo[]
  created_at?: string
}

export interface AccountInfo {
  id: number
  channel_type: string
  channel_user_id: string
  username?: string
  linked_at: string
}

export interface UserRole {
  id: number
  role_name: string
  role_type: string
  scope?: string
  granted_at: string
}

// === POSITIONS ===

export interface RefItem {
  id: number
  code: string
  name: string
}

export interface PersonInfo {
  id: number
  external_id?: string
  last_name: string
  first_name: string
  middle_name?: string
  email?: string
  phone?: string
}

export interface ImportedStudentInfo {
  person_id: number
  global_user_id?: number
  external_id?: string
  last_name: string
  first_name: string
  middle_name?: string
  email?: string
  phone?: string
  program_name?: string
  stream_name?: string
  study_group_name?: string
  status: string
  imported_via_excel: boolean
}

export interface SubgroupBrief {
  id: number
  name: string
}

export interface StudentPositionInfo {
  id: number
  program_id?: number
  program_name?: string
  stream_id?: number
  stream_name?: string
  study_group_id?: number
  study_group_name?: string
  status: string
  nationality_type: string
  funding_type: string
  education_form: string
  subgroups?: SubgroupBrief[]
  department_id?: number
  faculty_id?: number
}

export interface TeacherPositionInfo {
  id: number
  department_id?: number
  department_name?: string
  position_title: string
  employment_type: string
  status: string
  faculty_id?: number
}

export interface AdminAppointmentInfo {
  id: number
  appointment_type: string
  scope_type: string
  scope_id?: number
  scope_name?: string
  status: string
}

export interface AllPositions {
  student: StudentPositionInfo[]
  teacher: TeacherPositionInfo[]
  admin: AdminAppointmentInfo[]
}

export interface AdminCredentialInfo {
  id: number
  global_user_id: number
  email: string
  created_at: string
  updated_at: string
}

export interface StudentImportError {
  row: number
  field?: string
  message: string
}

export interface StudentImportResult {
  total: number
  created: number
  updated: number
  skipped: number
  errors?: StudentImportError[]
}

export interface ManualStudentCreateRequest {
  external_id: string
  last_name: string
  first_name: string
  middle_name?: string
  email?: string
  phone?: string
  program_code?: string
  stream_code?: string
  group_code: string
  subgroup_codes: string[]
  status: string
  nationality_type: string
  funding_type: string
  education_form: string
}

// Helpers for university reference CRUD — reduce per-entity boilerplate.
const refList = (res: string, param?: string) => (parentId?: number) => {
  const qs = param && parentId != null ? `?${param}=${parentId}` : ''
  return request<RefItem[]>(`/university/${res}${qs}`)
}
const refCreate = (res: string) => (data: Record<string, unknown>) =>
  request<{ id: number }>(`/university/manage/${res}`, { method: 'POST', body: JSON.stringify(data) })
const refUpdate = (res: string) => (id: number, data: Record<string, unknown>) =>
  request<{ status: string }>(`/university/manage/${res}/${id}`, { method: 'PUT', body: JSON.stringify(data) })
const refDelete = (res: string) => (id: number) =>
  request<{ status: string }>(`/university/manage/${res}/${id}`, { method: 'DELETE' })

export const api = {
  listPlugins: () => request<PluginInfo[]>('/plugins'),

  getPlugin: (id: string) => request<PluginDetail>(`/plugins/${encodeURIComponent(id)}`),

  uploadPlugin: (file: File) => {
    const form = new FormData()
    form.append('wasm', file)
    return request<PluginMeta>('/plugins/upload', { method: 'POST', body: form })
  },

  installPlugin: (id: string, body: { wasm_key: string; config: unknown }) =>
    request<{ id: string; status: string }>(`/plugins/${encodeURIComponent(id)}/install`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  updateConfig: (id: string, config: unknown) =>
    request<{ status: string }>(`/plugins/${encodeURIComponent(id)}/config`, {
      method: 'PUT',
      body: JSON.stringify({ config }),
    }),

  previewPluginUpdate: (id: string, file: File) => {
    const form = new FormData()
    form.append('wasm', file)
    return request<PluginUpdatePreviewResponse>(`/plugins/${encodeURIComponent(id)}/update/preview`, {
      method: 'POST',
      body: form,
    })
  },

  updatePlugin: (id: string, file: File, changelog?: string) => {
    const form = new FormData()
    form.append('wasm', file)
    if (changelog && changelog.trim()) {
      form.append('changelog', changelog.trim())
    }
    return request<{ status: string }>(`/plugins/${encodeURIComponent(id)}/update`, {
      method: 'POST',
      body: form,
    })
  },

  disablePlugin: (id: string) =>
    request<{ status: string }>(`/plugins/${encodeURIComponent(id)}/disable`, { method: 'POST' }),

  enablePlugin: (id: string) =>
    request<{ status: string }>(`/plugins/${encodeURIComponent(id)}/enable`, { method: 'POST' }),

  deletePlugin: (id: string) =>
    request<{ status: string }>(`/plugins/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  getRuleSchema: () => request<RuleSchema>('/rule-schema'),

  listCommandSettings: (pluginId: string) =>
    request<CommandSetting[]>(`/plugins/${encodeURIComponent(pluginId)}/commands/settings`),

  getPluginFrontendOrigins: (pluginId: string) =>
    request<PluginFrontendOrigins>(`/plugins/${encodeURIComponent(pluginId)}/frontend-origins`),

  setCommandEnabled: (pluginId: string, commandName: string, enabled: boolean) =>
    request<{ status: string }>(
      `/plugins/${encodeURIComponent(pluginId)}/commands/${encodeURIComponent(commandName)}/enabled`,
      { method: 'PUT', body: JSON.stringify({ enabled }) },
    ),

  setCommandAccess: (pluginId: string, commandName: string, access: { allow_user_keys: boolean; allow_service_keys: boolean }) =>
    request<{ status: string }>(
      `/plugins/${encodeURIComponent(pluginId)}/commands/${encodeURIComponent(commandName)}/access`,
      { method: 'PUT', body: JSON.stringify(access) },
    ),

  setCommandPolicy: (pluginId: string, commandName: string, expression: string) =>
    request<{ status: string }>(
      `/plugins/${encodeURIComponent(pluginId)}/commands/${encodeURIComponent(commandName)}/policy`,
      { method: 'PUT', body: JSON.stringify({ expression }) },
    ),

  getPluginPolicy: (pluginId: string) =>
    request<{ expression: string }>(`/plugins/${encodeURIComponent(pluginId)}/policy`),

  setPluginPolicy: (pluginId: string, expression: string) =>
    request<{ status: string }>(
      `/plugins/${encodeURIComponent(pluginId)}/policy`,
      { method: 'PUT', body: JSON.stringify({ expression }) },
    ),

  setPluginFrontendOrigins: (pluginId: string, allowedOrigins: string[]) =>
    request<{ status: string }>(
      `/plugins/${encodeURIComponent(pluginId)}/frontend-origins`,
      { method: 'PUT', body: JSON.stringify({ allowed_origins: allowedOrigins }) },
    ),

  setCommandOrigins: (pluginId: string, commandName: string, allowedOrigins: string[]) =>
    request<{ status: string }>(
      `/plugins/${encodeURIComponent(pluginId)}/commands/${encodeURIComponent(commandName)}/origins`,
      { method: 'PUT', body: JSON.stringify({ allowed_origins: allowedOrigins }) },
    ),

  listServiceKeys: () =>
    request<ServiceKeyInfo[]>('/http/service-keys'),

  createServiceKey: (body: { name: string; scopes: ServiceKeyScope[]; expires_at?: string }) =>
    request<CreatedServiceKey>('/http/service-keys', {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  deleteServiceKey: (id: number) =>
    request<{ status: string }>(`/http/service-keys/${id}`, { method: 'DELETE' }),

  getPluginRequirements: (id: string) =>
    request<PluginRequirementsDetail>(`/plugins/${encodeURIComponent(id)}/requirements`),

  listVersions: (pluginId: string) =>
    request<VersionInfo[]>(`/plugins/${encodeURIComponent(pluginId)}/versions`),

  rollbackVersion: (pluginId: string, versionId: number) =>
    request<{ status: string; version: string; version_id: number }>(
      `/plugins/${encodeURIComponent(pluginId)}/versions/${versionId}/rollback`,
      { method: 'POST' },
    ),

  deleteVersion: (pluginId: string, versionId: number) =>
    request<{ status: string }>(
      `/plugins/${encodeURIComponent(pluginId)}/versions/${versionId}`,
      { method: 'DELETE' },
    ),

  listChannelStatus: () => request<ChannelStatus[]>('/channels/status'),

  listChats: (params?: { channel_type?: string; chat_kind?: string }) => {
    const q = new URLSearchParams()
    if (params?.channel_type) q.set('channel_type', params.channel_type)
    if (params?.chat_kind) q.set('chat_kind', params.chat_kind)
    const qs = q.toString()
    return request<ChatReference[]>(`/chats${qs ? `?${qs}` : ''}`)
  },

  broadcast: (chatIds: number[], text: string) =>
    request<BroadcastResult[]>('/broadcast', {
      method: 'POST',
      body: JSON.stringify({ chat_ids: chatIds, text }),
    }),

  // === USERS ===

  listUsers: (params?: { search?: string; role?: string; channel?: string; offset?: number; limit?: number }) => {
    const q = new URLSearchParams()
    if (params?.search) q.set('search', params.search)
    if (params?.role) q.set('role', params.role)
    if (params?.channel) q.set('channel', params.channel)
    if (params?.offset) q.set('offset', String(params.offset))
    if (params?.limit) q.set('limit', String(params.limit))
    return request<{ users: UserListItem[]; total: number }>(`/users${q.toString() ? '?' + q : ''}`)
  },

  getUser: (id: number) => request<UserDetail>(`/users/${id}`),

  updateUser: (id: number, data: { locale?: string; role?: string }) =>
      request<{ status: string }>(`/users/${id}`, { method: 'PUT', body: JSON.stringify(data) }),

  deleteUser: (id: number) =>
      request<{ status: string }>(`/users/${id}`, { method: 'DELETE' }),

  importStudents: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return request<StudentImportResult>('/import/students', { method: 'POST', body: form }).then((result) => ({
      ...result,
      errors: result.errors ?? [],
    }))
  },

  createImportedStudent: (data: ManualStudentCreateRequest) =>
      request<{ status: string; created: boolean }>('/import/students/manual', {
        method: 'POST',
        body: JSON.stringify(data),
      }),

  getUserRoles: (userId: number) =>
      request<UserRole[]>(`/users/${userId}/roles`),

  removeUserRole: (userId: number, roleName: string, roleType: string) =>
      request<{ status: string }>(`/users/${userId}/roles`, {
        method: 'DELETE',
        body: JSON.stringify({ role_name: roleName, role_type: roleType }),
      }),

  unlinkAccount: (userId: number, accountId: number) =>
      request<{ status: string }>(`/users/${userId}/accounts/${accountId}`, { method: 'DELETE' }),

  // === UNIVERSITY REFERENCE DATA + CRUD ===

  listFaculties: refList('faculties'),
  createFaculty: refCreate('faculties'),
  updateFaculty: refUpdate('faculties'),
  deleteFaculty: refDelete('faculties'),

  listDepartments: refList('departments', 'faculty_id'),
  createDepartment: refCreate('departments'),
  updateDepartment: refUpdate('departments'),
  deleteDepartment: refDelete('departments'),

  listPrograms: refList('programs', 'department_id'),
  createProgram: refCreate('programs'),
  updateProgram: refUpdate('programs'),
  deleteProgram: refDelete('programs'),

  listStreams: refList('streams', 'program_id'),
  createStream: refCreate('streams'),
  updateStream: refUpdate('streams'),
  deleteStream: refDelete('streams'),

  listGroups: refList('groups', 'stream_id'),
  createGroup: refCreate('groups'),
  updateGroup: refUpdate('groups'),
  deleteGroup: refDelete('groups'),

  listSubgroups: refList('subgroups', 'study_group_id'),
  createSubgroup: refCreate('subgroups'),
  updateSubgroup: refUpdate('subgroups'),
  deleteSubgroup: refDelete('subgroups'),

  listCourses: refList('courses'),
  createCourse: refCreate('courses'),
  updateCourse: refUpdate('courses'),
  deleteCourse: refDelete('courses'),

  listSemesters: () => request<{ id: number; year: number; semester_type: string }[]>('/university/semesters'),
  createSemester: (data: { year: number; semester_type: string }) =>
      request<{ id: number }>('/university/manage/semesters', { method: 'POST', body: JSON.stringify(data) }),
  updateSemester: (id: number, data: { year: number; semester_type: string }) =>
      request<{ status: string }>(`/university/manage/semesters/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteSemester: (id: number) => request<{ status: string }>(`/university/manage/semesters/${id}`, { method: 'DELETE' }),

  // === PERSON & POSITIONS ===

  getUserPerson: (userId: number) => request<PersonInfo | null>(`/users/${userId}/person`),

  searchUnlinkedPersons: (query: string) => request<PersonInfo[]>(`/persons/search?q=${encodeURIComponent(query)}`),

  listImportedStudents: (query?: string) =>
      request<ImportedStudentInfo[]>(`/persons/imported${query ? `?q=${encodeURIComponent(query)}` : ''}`),

  linkPersonToUser: (userId: number, personId: number) =>
      request<{ status: string }>(`/users/${userId}/person/link`, { method: 'POST', body: JSON.stringify({ person_id: personId }) }),

  createUserPerson: (userId: number, data: Omit<PersonInfo, 'id'>) =>
      request<PersonInfo>(`/users/${userId}/person`, { method: 'POST', body: JSON.stringify(data) }),

  getUserPositions: (userId: number) => request<AllPositions>(`/users/${userId}/positions`),

  createStudentPosition: (userId: number, data: Omit<StudentPositionInfo, 'id' | 'program_name' | 'stream_name' | 'study_group_name' | 'department_id' | 'faculty_id'>) =>
      request<StudentPositionInfo>(`/users/${userId}/positions/student`, { method: 'POST', body: JSON.stringify(data) }),
  updateStudentPosition: (userId: number, posId: number, data: Omit<StudentPositionInfo, 'id' | 'program_name' | 'stream_name' | 'study_group_name' | 'department_id' | 'faculty_id'>) =>
      request<{ status: string }>(`/users/${userId}/positions/student/${posId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteStudentPosition: (userId: number, posId: number) =>
      request<{ status: string }>(`/users/${userId}/positions/student/${posId}`, { method: 'DELETE' }),

  createTeacherPosition: (userId: number, data: Omit<TeacherPositionInfo, 'id' | 'department_name' | 'faculty_id'>) =>
      request<TeacherPositionInfo>(`/users/${userId}/positions/teacher`, { method: 'POST', body: JSON.stringify(data) }),
  updateTeacherPosition: (userId: number, posId: number, data: Omit<TeacherPositionInfo, 'id' | 'department_name' | 'faculty_id'>) =>
      request<{ status: string }>(`/users/${userId}/positions/teacher/${posId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteTeacherPosition: (userId: number, posId: number) =>
      request<{ status: string }>(`/users/${userId}/positions/teacher/${posId}`, { method: 'DELETE' }),

  createAdminAppointment: (userId: number, data: Omit<AdminAppointmentInfo, 'id' | 'scope_name'>) =>
      request<AdminAppointmentInfo>(`/users/${userId}/positions/admin-appointment`, { method: 'POST', body: JSON.stringify(data) }),
  updateAdminAppointment: (userId: number, posId: number, data: Omit<AdminAppointmentInfo, 'id' | 'scope_name'>) =>
      request<{ status: string }>(`/users/${userId}/positions/admin-appointment/${posId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteAdminAppointment: (userId: number, posId: number) =>
      request<{ status: string }>(`/users/${userId}/positions/admin-appointment/${posId}`, { method: 'DELETE' }),

  // === ADMIN CREDENTIALS ===

  listAdminCredentials: () => request<AdminCredentialInfo[]>('/admins'),

  getAdminCredential: (userId: number) =>
      request<{ has_access: boolean; credential?: AdminCredentialInfo }>(`/admins/${userId}`),

  createAdminCredential: (data: { global_user_id: number; email: string; password?: string }) =>
      request<AdminCredentialInfo>('/admins', { method: 'POST', body: JSON.stringify(data) }),

  updateAdminPassword: (userId: number, password: string) =>
      request<{ status: string }>(`/admins/${userId}/password`, { method: 'PUT', body: JSON.stringify({ password }) }),

  updateAdminEmail: (userId: number, email: string) =>
      request<{ status: string }>(`/admins/${userId}/email`, { method: 'PUT', body: JSON.stringify({ email }) }),

  deleteAdminCredential: (userId: number) =>
      request<{ status: string }>(`/admins/${userId}`, { method: 'DELETE' }),

  changeOwnPassword: (currentPassword: string, newPassword: string) =>
      fetch('/api/admin/auth/password', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
      }).then(async (res) => {
        const data = await res.json()
        if (!res.ok) throw new ApiError(data.error ?? `HTTP ${res.status}`, res.status)
        return data as { status: string }
      }),

  listOwnTokens: () =>
      requestWithBase<UserTokenInfo[]>('', '/api/auth/tokens'),

  createOwnToken: (body: { name: string; expires_at?: string }) =>
      requestWithBase<CreatedUserToken>('', '/api/auth/tokens', {
        method: 'POST',
        body: JSON.stringify(body),
      }),

  deleteOwnToken: (id: number) =>
      requestWithBase<{ status: string }>('', `/api/auth/tokens/${id}`, { method: 'DELETE' }),

  listStudentTrips: () =>
      requestWithBase<StudentTripsResponse>('', '/api/triggers/http/student-location/api/trips'),

  requestStudentTripFix: (id: number, reason: string) =>
      requestWithBase<StudentTripActionResponse>('', '/api/triggers/http/student-location/api/trips/action', {
        method: 'POST',
        body: JSON.stringify({ id, action: 'request_fix', reason }),
      }),
}

// ─── Dean's office API ────────────────────────────────────────────────────────

const DEAN_BASE = '/api/dean'

async function deanRequest<T>(url: string, init?: RequestInit): Promise<T> {
  return requestWithBase<T>(DEAN_BASE, url, init)
}

export interface DeanMe {
  faculty_id: number
  faculty_name: string
  faculty_code: string
}

export interface FacultyStats {
  faculty_id: number
  faculty_name: string
  faculty_code: string
  group_count: number
  active_students: number
  budget_students: number
  contract_students: number
  foreign_students: number
}

export interface GroupStats {
  group_id: number
  group_code: string
  group_name: string
  program_name: string
  active_students: number
  budget_students: number
  contract_students: number
  foreign_students: number
}

export interface DeanDashboard {
  faculty: FacultyStats
  groups: GroupStats[]
}

export interface StudentRow {
  person_id: number
  external_id?: string
  last_name: string
  first_name: string
  middle_name?: string
  email?: string
  phone?: string
  bot_user_id?: number
  position_id: number
  status: string
  nationality_type: string
  funding_type: string
  education_form: string
  group_id?: number
  group_code?: string
  program_name?: string
}

export interface GroupBrief {
  id: number
  code: string
  name: string
}

export interface UpdateStudentBody {
  study_group_id?: number | null
  status: string
  nationality_type: string
  funding_type: string
  education_form: string
  email?: string
  phone?: string
}

export const deanApi = {
  getMe: () => deanRequest<DeanMe>('/me'),

  getDashboard: () => deanRequest<DeanDashboard>('/dashboard'),

  listGroups: () => deanRequest<GroupBrief[]>('/groups'),

  listStudents: (params?: {
    group_id?: number
    status?: string
    nationality_type?: string
    funding_type?: string
    search?: string
  }) => {
    const q = new URLSearchParams()
    if (params?.group_id) q.set('group_id', String(params.group_id))
    if (params?.status) q.set('status', params.status)
    if (params?.nationality_type) q.set('nationality_type', params.nationality_type)
    if (params?.funding_type) q.set('funding_type', params.funding_type)
    if (params?.search) q.set('search', params.search)
    const qs = q.toString()
    return deanRequest<StudentRow[]>(`/students${qs ? `?${qs}` : ''}`)
  },

  getStudent: (positionId: number) =>
    deanRequest<StudentRow>(`/students/${positionId}`),

  updateStudent: (positionId: number, body: UpdateStudentBody) =>
    deanRequest<{ status: string }>(`/students/${positionId}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
}
