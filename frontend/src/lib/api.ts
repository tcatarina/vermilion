import type { ProjectItem, BoardPayload, BoardIssue, StatusItem, IssueDetail, ApiError } from './types'

export class ApiRequestError extends Error {
  constructor(
    public readonly status: number,
    public readonly error: ApiError,
  ) {
    super(error.message)
  }
}

function headers(projectIdentifier?: string): Record<string, string> {
  const h: Record<string, string> = {
    'Content-Type': 'application/json',
    'X-Redmine-URL': localStorage.getItem('vermilion_redmine_url') || '',
    'X-Redmine-API-Key': localStorage.getItem('vermilion_api_key') || '',
  }
  if (projectIdentifier) {
    h['X-Redmine-Project-Identifier'] = projectIdentifier
  }
  return h
}

async function call<T>(method: string, path: string, body?: unknown, projectIdentifier?: string): Promise<T> {
  const resp = await fetch(`/api${path}`, {
    method,
    headers: headers(projectIdentifier),
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if (resp.status === 204) return undefined as T
  const data = await resp.json().catch(() => ({ error: { code: 'PARSE_ERROR', message: 'invalid response' } }))
  if (!resp.ok) {
    throw new ApiRequestError(resp.status, data.error ?? { code: 'UNKNOWN', message: resp.statusText })
  }
  return data as T
}

export const api = {
  listProjects: () =>
    call<ProjectItem[]>('GET', '/projects'),

  listStatuses: () =>
    call<StatusItem[]>('GET', '/board/statuses'),

  loadBoard: (project: string, force = false) =>
    call<BoardPayload>('GET', `/board?project=${encodeURIComponent(project)}${force ? '&force=true' : ''}`, undefined, project),

  getIssue: (issueId: number, project?: string) =>
    call<IssueDetail>('GET', `/issues/${issueId}`, undefined, project),

  moveIssue: (issueId: number, statusId: number, project?: string) =>
    call<BoardIssue>('PUT', `/issues/${issueId}/status`, { statusId }, project),

  postComment: (issueId: number, notes: string, project?: string) =>
    call<IssueDetail>('POST', `/issues/${issueId}/comments`, { notes }, project),
}
