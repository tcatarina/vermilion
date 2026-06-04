<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useConfigStore } from '../stores/config'
import { useBoardStore } from '../stores/board'
import { usePresetsStore } from '../stores/presets'
import KanbanBoard from '../components/KanbanBoard.vue'
import IssueModal from '../components/IssueModal.vue'
import AppSelect from '../components/AppSelect.vue'
import type { BoardIssue } from '../lib/types'

const config = useConfigStore()
const board = useBoardStore()
const presetsStore = usePresetsStore()
const router = useRouter()

const openIssueId = ref<number | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null
let suppressPresetClear = false

const sortedMembers = computed(() => board.members.slice().sort((a, b) => a.name.localeCompare(b.name)))
const sortedVersions = computed(() => board.versions.slice().sort((a, b) => a.name.localeCompare(b.name)))
const sortedProjects = computed(() => config.projects.slice().sort((a, b) => a.name.localeCompare(b.name)))
const sortedPresets = computed(() => presetsStore.presets.slice().sort((a, b) => a.name.localeCompare(b.name)))

const orderedStatuses = computed(() => {
  const order = config.columnOrder
  let statuses = order.length
    ? order.map(id => board.allStatuses.find(s => s.id === id)).filter(Boolean) as typeof board.allStatuses
    : board.allStatuses
  const preset = presetsStore.activePreset
  if (preset && preset.statusIds.length > 0) {
    statuses = statuses.filter(s => preset.statusIds.includes(s.id))
  }
  return statuses.slice().sort((a, b) => a.name.localeCompare(b.name))
})

function startPolling() {
  stopPolling()
  pollTimer = setInterval(() => {
    if (config.selectedProject) board.loadBoard(config.selectedProject)
  }, 60000)
}

function stopPolling() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
}

async function refresh(force = false) {
  if (config.selectedProject) await board.loadBoard(config.selectedProject, force)
}

function applyOrClearPreset(id: string) {
  if (!id) {
    clearPreset()
    return
  }
  presetsStore.applyPreset(id)
  const preset = presetsStore.activePreset
  if (!preset) return
  board.resetFilters()
  board.filterAssigneeIds = [...preset.assigneeIds]
  board.filterVersionIds = [...preset.versionIds]
  suppressPresetClear = true
  config.setProject(preset.projectIdentifier)
}

function clearPreset() {
  presetsStore.clearPreset()
  board.resetPresetFilters()
}

onMounted(async () => {
  if (!config.isReady) { router.push('/setup'); return }
  if (config.projects.length === 0) await config.fetchProjects()
  // Restore active preset filters
  const preset = presetsStore.activePreset
  if (preset) {
    board.filterAssigneeIds = preset.assigneeIds
    board.filterVersionIds = preset.versionIds
  }
  await refresh()
  startPolling()
})

onUnmounted(stopPolling)

watch(() => config.selectedProject, () => {
  board.resetFilters()
  if (!suppressPresetClear) {
    board.resetPresetFilters()
    presetsStore.clearPreset()
  }
  suppressPresetClear = false
  refresh()
})

function handleDrop(issueId: number, newStatusId: number) {
  board.moveIssue(issueId, newStatusId).catch(() => {})
}
</script>

<template>
  <div class="flex flex-col h-screen overflow-hidden bg-black">

    <!-- Toolbar -->
    <header class="flex items-center gap-3 px-4 py-2.5 bg-zinc-950 border-b border-white/10 flex-shrink-0">
      <!-- Left: brand + project + preset -->
      <div class="flex items-center gap-2 flex-shrink-0">
        <h1 class="text-lg font-bold text-white tracking-tight">Vermilion</h1>
        <AppSelect
          :model-value="config.selectedProject"
          :options="[{ value: '', label: 'Select project...' }, ...sortedProjects.map(p => ({ value: p.identifier, label: p.name }))]"
          :disabled="!!presetsStore.activePresetId"
          @update:model-value="config.setProject($event)"
        />
        <AppSelect
          :model-value="presetsStore.activePresetId ?? ''"
          :options="[{ value: '', label: 'No preset' }, ...sortedPresets.map(p => ({ value: p.id, label: p.name }))]"
          @update:model-value="applyOrClearPreset($event)"
        />
        <button
          v-if="presetsStore.activePresetId"
          @click="clearPreset"
          title="Clear preset"
          class="p-1.5 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors text-xs"
        >✕</button>
      </div>

      <div class="flex-1" />

      <!-- Right: filters + status + actions -->
      <div class="flex items-center gap-2 flex-shrink-0">
        <AppSelect
          v-model="board.filterAssigneeId"
          :options="[{ value: null, label: 'All assignees' }, ...sortedMembers.map(m => ({ value: m.id, label: m.name }))]"
          :disabled="!!presetsStore.activePresetId"
        />
        <AppSelect
          v-model="board.filterVersionId"
          :options="[{ value: null, label: 'All versions' }, ...sortedVersions.map(v => ({ value: v.id, label: v.name }))]"
          :disabled="!!presetsStore.activePresetId"
        />
        <template v-if="board.currentUserId !== null">
          <div class="w-px h-4 bg-white/10" />
          <span class="text-xs text-zinc-400 font-medium">
            {{ board.members.find(m => m.id === board.currentUserId)?.name ?? 'You' }}
          </span>
        </template>
        <span v-if="board.stale" class="text-xs bg-zinc-800 text-zinc-300 px-2 py-0.5 rounded-full font-medium">STALE</span>
        <button
          @click="refresh(true)"
          :disabled="board.loading"
          class="p-1.5 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 disabled:opacity-30 transition-colors"
          title="Refresh"
        >⟳</button>
        <button
          @click="router.push('/setup')"
          class="p-1.5 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors"
          title="Settings"
        >⚙</button>
      </div>
    </header>

    <!-- Main area: sidebar + board -->
    <div class="flex flex-1 overflow-hidden min-h-0">

      <!-- Left sidebar -->
      <nav class="flex flex-col items-center py-3 gap-2 bg-zinc-950 border-r border-white/10 w-11 flex-shrink-0">
        <button
          @click="router.push('/presets')"
          title="Presets"
          class="p-2 rounded-lg text-zinc-500 hover:text-white hover:bg-zinc-800 transition-colors"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="4" y1="6" x2="20" y2="6"/>
            <line x1="8" y1="12" x2="20" y2="12"/>
            <line x1="12" y1="18" x2="20" y2="18"/>
            <circle cx="4" cy="12" r="1" fill="currentColor" stroke="none"/>
            <circle cx="4" cy="18" r="1" fill="currentColor" stroke="none"/>
          </svg>
        </button>
      </nav>

      <!-- Board column -->
      <div class="flex flex-col flex-1 overflow-hidden min-h-0">

        <!-- Error bar -->
        <div v-if="board.error" class="px-4 py-2 bg-zinc-900 text-zinc-300 text-sm border-b border-white/10 flex-shrink-0">
          {{ board.error }}
        </div>

        <!-- Loading -->
        <div v-if="board.loading && !board.allStatuses.length" class="flex-1 flex items-center justify-center text-zinc-600">
          Loading board...
        </div>

        <!-- Empty: no project -->
        <div v-else-if="!config.selectedProject" class="flex-1 flex items-center justify-center text-zinc-600">
          Select a project to view its board
        </div>

        <!-- Board -->
        <KanbanBoard
          v-else-if="orderedStatuses.length"
          :statuses="orderedStatuses"
          :issues-by-status="board.issuesForStatus"
          :highlight-color="config.cardColor"
          :current-user-id="board.currentUserId"
          :redmine-url="config.redmineUrl"
          @card-click="(issue: BoardIssue) => openIssueId = issue.id"
          @status-change="handleDrop"
          class="flex-1 overflow-hidden"
        />

      </div>
    </div>

    <!-- Issue modal -->
    <IssueModal
      v-if="openIssueId !== null"
      :issue-id="openIssueId"
      :project-identifier="config.selectedProject"
      @close="openIssueId = null"
    />
  </div>
</template>
