export interface ProjectItem {
  id: number
  name: string
  identifier: string
}

export interface StatusItem {
  id: number
  name: string
  isClosed: boolean
}

export interface MemberItem {
  id: number
  name: string
}

export interface VersionItem {
  id: number
  name: string
}

export interface BoardIssue {
  id: number
  subject: string
  statusId: number
  assignedToId: number | null
  assignedToName: string
  updatedOn: string
  versionId: number | null
  versionName: string
  spentHours: number
  estimatedHours: number
}

export interface BoardProject {
  id: number
  name: string
  identifier: string
}

export interface BoardPayload {
  project: BoardProject
  statuses: StatusItem[]
  members: MemberItem[]
  versions: VersionItem[]
  issues: BoardIssue[]
  cachedAt: string
  stale: boolean
  currentUserId: number | null
}

export interface JournalDetail {
  property: string
  name: string
  oldValue: string
  newValue: string
}

export interface Journal {
  id: number
  userName: string
  notes: string
  createdOn: string
  details: JournalDetail[]
}

export interface IssueDetail {
  id: number
  subject: string
  description: string
  statusId: number
  statusName: string
  assignedToId: number | null
  assignedToName: string
  updatedOn: string
  createdOn: string
  versionId: number | null
  versionName: string
  spentHours: number
  estimatedHours: number
  journals: Journal[]
}

export interface Preset {
  id: string
  name: string
  projectIdentifier: string
  assigneeIds: number[]
  versionIds: number[]
  statusIds: number[]
}

export interface ApiError {
  code: string
  message: string
  details?: Record<string, string>
}
