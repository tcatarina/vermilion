<script setup lang="ts">
import type { BoardIssue } from '../lib/types'

const props = defineProps<{
  issue: BoardIssue
  isMyCard: boolean
  highlightColor: string
  redmineUrl: string
}>()

const emit = defineEmits<{
  (e: 'click'): void
  (e: 'dragstart', id: number): void
}>()

function initials(name: string): string {
  return name.split(' ').map(p => p[0]).slice(0, 2).join('').toUpperCase()
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

function yiqTextColor(hex: string): string {
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return (r * 299 + g * 587 + b * 114) / 1000 >= 128 ? '#000000' : '#ffffff'
}

function formatHours(h: number): string {
  if (!h) return ''
  const hours = Math.floor(h)
  const mins = Math.round((h - hours) * 60)
  if (hours && mins) return `${hours}h ${mins}m`
  if (hours) return `${hours}h`
  return `${mins}m`
}


function handleDragStart(e: DragEvent) {
  e.dataTransfer!.effectAllowed = 'move'
  e.dataTransfer!.setData('text/plain', String(props.issue.id))
  emit('dragstart', props.issue.id)
}
</script>

<template>
  <div
    draggable="true"
    @dragstart="handleDragStart"
    @click="emit('click')"
    class="rounded-lg border p-3 cursor-pointer hover:border-white/30 transition-colors select-none"
    :style="isMyCard
      ? { backgroundColor: highlightColor, color: yiqTextColor(highlightColor), borderColor: 'transparent' }
      : {}"
    :class="isMyCard ? '' : 'bg-zinc-900 border-white/10'"
  >
    <!-- ID + time + external link -->
    <div class="flex items-center justify-between mb-2">
      <span class="text-xs font-mono text-zinc-500">#{{ issue.id }}</span>
      <div class="flex items-center gap-1.5">
        <span v-if="issue.spentHours" class="text-xs text-zinc-500">⏱ {{ formatHours(issue.spentHours) }}</span>
        <a
          v-if="redmineUrl"
          :href="`${redmineUrl}/issues/${issue.id}`"
          target="_blank"
          rel="noopener"
          @click.stop
          class="text-xs text-zinc-500 hover:text-white transition-colors"
          title="Open in Redmine"
        >↗</a>
      </div>
    </div>

    <!-- Subject -->
    <p class="text-sm font-medium leading-snug line-clamp-2 mb-2">{{ issue.subject }}</p>

    <!-- Footer -->
    <div class="flex items-center justify-between mt-1">
      <div class="flex items-center gap-1.5">
        <div
          class="w-6 h-6 rounded-full flex items-center justify-center text-xs font-semibold"
          :class="isMyCard ? 'bg-black/20' : 'bg-zinc-700 text-zinc-300'"
        >
          {{ issue.assignedToName ? initials(issue.assignedToName) : '?' }}
        </div>
        <span class="text-xs text-zinc-400 truncate max-w-[80px]">
          {{ issue.assignedToName || 'Unassigned' }}
        </span>
      </div>
      <span class="text-xs text-zinc-600">{{ formatDate(issue.updatedOn) }}</span>
    </div>

    <!-- Version badge -->
    <div v-if="issue.versionName" class="mt-2">
      <span
        class="text-xs px-1.5 py-0.5 rounded-full"
        :class="isMyCard ? 'bg-black/20' : 'bg-zinc-800 text-zinc-400'"
      >{{ issue.versionName }}</span>
    </div>
  </div>
</template>
