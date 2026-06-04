import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { BoardPayload, BoardIssue, StatusItem, MemberItem, VersionItem, BoardProject } from '../lib/types'
import { api } from '../lib/api'

export const useBoardStore = defineStore('board', () => {
  const project = ref<BoardProject | null>(null)
  const allStatuses = ref<StatusItem[]>([])
  const members = ref<MemberItem[]>([])
  const versions = ref<VersionItem[]>([])
  const issues = ref<BoardIssue[]>([])
  const cachedAt = ref('')
  const stale = ref(false)
  const loading = ref(false)
  const error = ref('')

  const filterAssigneeId = ref<number | null>(null)
  const filterVersionId = ref<number | null>(null)
  const filterAssigneeIds = ref<number[]>([])
  const filterVersionIds = ref<number[]>([])
  const currentUserId = ref<number | null>(null)

  const filteredIssues = computed(() => {
    let result = issues.value
    if (filterAssigneeIds.value.length > 0) {
      result = result.filter(i => i.assignedToId !== null && filterAssigneeIds.value.includes(i.assignedToId))
    } else if (filterAssigneeId.value !== null) {
      result = result.filter(i => i.assignedToId === filterAssigneeId.value)
    }
    if (filterVersionIds.value.length > 0) {
      result = result.filter(i => i.versionId !== null && filterVersionIds.value.includes(i.versionId))
    } else if (filterVersionId.value !== null) {
      result = result.filter(i => i.versionId === filterVersionId.value)
    }
    return result
  })

  function issuesForStatus(statusId: number): BoardIssue[] {
    return filteredIssues.value.filter(i => i.statusId === statusId)
  }

  async function loadBoard(projectIdentifier: string, force = false) {
    loading.value = true
    error.value = ''
    try {
      const data: BoardPayload = await api.loadBoard(projectIdentifier, force)
      project.value = data.project
      allStatuses.value = data.statuses
      members.value = data.members
      versions.value = data.versions
      issues.value = data.issues
      cachedAt.value = data.cachedAt
      stale.value = data.stale
      currentUserId.value = data.currentUserId ?? null
    } catch (e: any) {
      error.value = e.message || 'Failed to load board'
    } finally {
      loading.value = false
    }
  }

  async function moveIssue(issueId: number, newStatusId: number) {
    const projectId = project.value?.identifier
    // Optimistic update
    const idx = issues.value.findIndex(i => i.id === issueId)
    const prev = idx >= 0 ? issues.value[idx].statusId : null
    if (idx >= 0) issues.value[idx] = { ...issues.value[idx], statusId: newStatusId }
    try {
      const updated = await api.moveIssue(issueId, newStatusId, projectId || undefined)
      if (idx >= 0) issues.value[idx] = updated
    } catch (e: any) {
      // Revert
      if (idx >= 0 && prev !== null) issues.value[idx] = { ...issues.value[idx], statusId: prev }
      error.value = e.message || 'Failed to move issue'
      throw e
    }
  }

  function resetFilters() {
    filterAssigneeId.value = null
    filterVersionId.value = null
  }

  function resetPresetFilters() {
    filterAssigneeIds.value = []
    filterVersionIds.value = []
  }

  return {
    project, allStatuses, members, versions, issues,
    cachedAt, stale, loading, error,
    filterAssigneeId, filterVersionId,
    filterAssigneeIds, filterVersionIds,
    currentUserId,
    filteredIssues, issuesForStatus,
    loadBoard, moveIssue, resetFilters, resetPresetFilters,
  }
})
