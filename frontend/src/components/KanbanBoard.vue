<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import type { BoardIssue, StatusItem } from '../lib/types'
import KanbanCard from './KanbanCard.vue'

const props = defineProps<{
  statuses: StatusItem[]
  issuesByStatus: (statusId: number) => BoardIssue[]
  highlightColor: string
  currentUserId: number | null
  redmineUrl: string
}>()

const emit = defineEmits<{
  (e: 'card-click', issue: BoardIssue): void
  (e: 'status-change', issueId: number, newStatusId: number): void
  (e: 'column-reorder', fromStatusId: number, toStatusId: number): void
}>()

// --- card drag state ---
const dragOverStatusId = ref<number | null>(null)
let draggingIssueId: number | null = null

// --- column drag state ---
const draggingColumnId = ref<number | null>(null)
const dragOverColumnId = ref<number | null>(null)

// --- column search state ---
const columnSearch = ref<Record<number, string>>({})
const columnSearchOpen = ref<Record<number, boolean>>({})
const columnRefs = ref<Record<number, HTMLElement | null>>({})

function toggleSearch(statusId: number) {
  columnSearchOpen.value[statusId] = !columnSearchOpen.value[statusId]
  if (!columnSearchOpen.value[statusId]) columnSearch.value[statusId] = ''
}

function onDocumentClick(e: MouseEvent) {
  props.statuses.forEach(status => {
    if (!columnSearchOpen.value[status.id]) return
    const el = columnRefs.value[status.id]
    if (el && !el.contains(e.target as Node)) {
      columnSearchOpen.value[status.id] = false
      columnSearch.value[status.id] = ''
    }
  })
}

onMounted(() => document.addEventListener('mousedown', onDocumentClick))
onUnmounted(() => document.removeEventListener('mousedown', onDocumentClick))

function filteredIssues(statusId: number): ReturnType<typeof props.issuesByStatus> {
  const q = (columnSearch.value[statusId] || '').toLowerCase().trim()
  const issues = props.issuesByStatus(statusId)
  return q ? issues.filter(i => i.subject.toLowerCase().includes(q)) : issues
}

// --- card drag handlers ---
function onDragStart(id: number) {
  draggingIssueId = id
}

function onDragOver(e: DragEvent, statusId: number) {
  e.preventDefault()
  e.dataTransfer!.dropEffect = 'move'
  if (draggingColumnId.value !== null) {
    dragOverColumnId.value = statusId
  } else {
    dragOverStatusId.value = statusId
  }
}

function onDragLeave(statusId: number) {
  if (draggingColumnId.value !== null) {
    if (dragOverColumnId.value === statusId) dragOverColumnId.value = null
  } else {
    if (dragOverStatusId.value === statusId) dragOverStatusId.value = null
  }
}

function onDrop(e: DragEvent, statusId: number) {
  e.preventDefault()
  if (draggingColumnId.value !== null) {
    dragOverColumnId.value = null
    if (draggingColumnId.value !== statusId) {
      emit('column-reorder', draggingColumnId.value, statusId)
    }
    draggingColumnId.value = null
  } else {
    dragOverStatusId.value = null
    const id = draggingIssueId ?? parseInt(e.dataTransfer!.getData('text/plain'))
    if (id) emit('status-change', id, statusId)
    draggingIssueId = null
  }
}

// --- column drag handlers ---
function onColumnDragStart(e: DragEvent, statusId: number) {
  draggingColumnId.value = statusId
  e.dataTransfer!.effectAllowed = 'move'
}

function onColumnDragEnd() {
  draggingColumnId.value = null
  dragOverColumnId.value = null
}
</script>

<template>
  <div class="flex gap-3 overflow-x-auto pb-4 flex-1 px-4 pt-4 min-h-0 bg-black">
    <div
      v-for="status in statuses"
      :key="status.id"
      :ref="el => columnRefs[status.id] = el as HTMLElement"
      class="flex flex-col w-72 flex-shrink-0 rounded-xl border transition-colors min-h-0 overflow-hidden"
      :class="dragOverColumnId === status.id && draggingColumnId !== status.id
        ? 'border-white/50 border-dashed'
        : dragOverStatusId === status.id
          ? 'border-white/30 border-dashed'
          : 'border-white/10'"
      @dragover="onDragOver($event, status.id)"
      @dragleave="onDragLeave(status.id)"
      @drop="onDrop($event, status.id)"
    >
      <!-- Column header -->
      <div class="flex items-center justify-between px-3 py-2 border-b border-white/10">
        <div class="flex items-center gap-1.5 min-w-0">
          <span
            draggable="true"
            class="text-zinc-700 hover:text-zinc-400 cursor-grab active:cursor-grabbing flex-shrink-0 select-none"
            title="Drag to reorder column"
            @dragstart="onColumnDragStart($event, status.id)"
            @dragend="onColumnDragEnd"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="w-2.5 h-3.5" viewBox="0 0 8 14" fill="currentColor">
              <circle cx="2" cy="2" r="1.5"/>
              <circle cx="6" cy="2" r="1.5"/>
              <circle cx="2" cy="7" r="1.5"/>
              <circle cx="6" cy="7" r="1.5"/>
              <circle cx="2" cy="12" r="1.5"/>
              <circle cx="6" cy="12" r="1.5"/>
            </svg>
          </span>
          <span class="text-sm font-semibold text-zinc-300 truncate">{{ status.name }}</span>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="text-xs bg-zinc-800 text-zinc-400 px-2 py-0.5 rounded-full">
            {{ filteredIssues(status.id).length }}
          </span>
          <button
            @click="toggleSearch(status.id)"
            class="p-1 rounded transition-colors"
            :class="columnSearchOpen[status.id] ? 'text-white' : 'text-zinc-600 hover:text-zinc-300'"
            title="Filter cards"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/>
            </svg>
          </button>
        </div>
      </div>
      <!-- Column search -->
      <div v-if="columnSearchOpen[status.id]" class="px-2 pt-2">
        <input
          v-model="columnSearch[status.id]"
          type="text"
          placeholder="Filter by title..."
          autofocus
          class="w-full px-2 py-1 text-xs bg-zinc-900 border border-white/10 rounded-md text-zinc-300 placeholder-zinc-600 focus:outline-none focus:ring-1 focus:ring-white/20"
        />
      </div>

      <!-- Drop zone -->
      <div
        class="flex-1 p-2 space-y-2 min-h-24 overflow-y-auto transition-colors"
        :class="dragOverStatusId === status.id && draggingColumnId === null ? 'bg-white/5' : 'bg-zinc-950'"
      >
        <KanbanCard
          v-for="issue in filteredIssues(status.id)"
          :key="issue.id"
          :issue="issue"
          :is-my-card="currentUserId !== null && issue.assignedToId === currentUserId"
          :highlight-color="highlightColor"
          :redmine-url="redmineUrl"
          @click="emit('card-click', issue)"
          @dragstart="onDragStart"
        />
        <div
          v-if="filteredIssues(status.id).length === 0"
          class="text-xs text-zinc-700 text-center py-4"
        >Drop here</div>
      </div>
    </div>
  </div>
</template>
