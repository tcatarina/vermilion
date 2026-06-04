<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import type { IssueDetail } from '../lib/types'
import { api } from '../lib/api'
import { useConfigStore } from '../stores/config'

const props = defineProps<{
  issueId: number
  projectIdentifier?: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'updated', issue: IssueDetail): void
}>()

const config = useConfigStore()
const issue = ref<IssueDetail | null>(null)
const loading = ref(false)
const error = ref('')
const commentText = ref('')
const submittingComment = ref(false)
const commentError = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    issue.value = await api.getIssue(props.issueId, props.projectIdentifier)
  } catch (e: any) {
    error.value = e.message || 'Failed to load issue'
  } finally {
    loading.value = false
  }
}

async function submitComment() {
  if (!commentText.value.trim()) return
  submittingComment.value = true
  commentError.value = ''
  try {
    issue.value = await api.postComment(props.issueId, commentText.value.trim(), props.projectIdentifier)
    commentText.value = ''
    emit('updated', issue.value)
  } catch (e: any) {
    commentError.value = e.message || 'Failed to add comment'
  } finally {
    submittingComment.value = false
  }
}

function formatHours(h: number): string {
  if (!h) return '—'
  const hours = Math.floor(h)
  const mins = Math.round((h - hours) * 60)
  if (hours && mins) return `${hours}h ${mins}m`
  if (hours) return `${hours}h`
  return `${mins}m`
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit'
  })
}

function formatPropName(name: string): string {
  const map: Record<string, string> = {
    status_id: 'Status',
    assigned_to_id: 'Assignee',
    fixed_version_id: 'Version',
    done_ratio: 'Progress',
    priority_id: 'Priority',
    subject: 'Subject',
    description: 'Description',
  }
  return map[name] || name
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}

onMounted(() => { load(); window.addEventListener('keydown', onKey) })
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <!-- Backdrop -->
    <div class="absolute inset-0 bg-black/80" @click="emit('close')" />

    <!-- Panel -->
    <div class="relative w-full max-w-2xl max-h-[90vh] bg-zinc-950 border border-white/10 rounded-xl shadow-2xl flex flex-col">

      <!-- Loading -->
      <div v-if="loading" class="flex-1 flex items-center justify-center p-12 text-zinc-600">
        Loading...
      </div>

      <!-- Error -->
      <div v-else-if="error" class="flex-1 flex flex-col items-center justify-center p-12 gap-4 text-zinc-400">
        <span>{{ error }}</span>
        <button @click="load" class="px-4 py-2 bg-zinc-800 hover:bg-zinc-700 text-white rounded-lg text-sm">Retry</button>
      </div>

      <template v-else-if="issue">
        <!-- Header -->
        <div class="flex items-start justify-between p-5 border-b border-white/10">
          <div>
            <div class="flex items-center gap-2 mb-1">
              <span class="text-xs font-mono text-zinc-600">#{{ issue.id }}</span>
              <a
                :href="`${config.redmineUrl}/issues/${issue.id}`"
                target="_blank" rel="noopener"
                class="text-xs text-zinc-500 hover:text-white transition-colors"
              >View in Redmine ↗</a>
            </div>
            <h2 class="text-xl font-semibold text-white leading-tight">{{ issue.subject }}</h2>
          </div>
          <button @click="emit('close')" class="text-zinc-600 hover:text-white text-2xl leading-none ml-4 transition-colors">×</button>
        </div>

        <!-- Body -->
        <div class="flex-1 overflow-y-auto p-5 space-y-5">

          <!-- Meta grid -->
          <div class="grid grid-cols-3 gap-3">
            <div class="bg-zinc-900 rounded-lg p-3 border border-white/5">
              <div class="text-xs text-zinc-600 mb-0.5">Status</div>
              <div class="text-sm font-medium text-white">{{ issue.statusName }}</div>
            </div>
            <div class="bg-zinc-900 rounded-lg p-3 border border-white/5">
              <div class="text-xs text-zinc-600 mb-0.5">Assignee</div>
              <div class="text-sm font-medium text-white">{{ issue.assignedToName || '—' }}</div>
            </div>
            <div class="bg-zinc-900 rounded-lg p-3 border border-white/5">
              <div class="text-xs text-zinc-600 mb-0.5">Version</div>
              <div class="text-sm font-medium text-white">{{ issue.versionName || '—' }}</div>
            </div>
            <div class="bg-zinc-900 rounded-lg p-3 border border-white/5">
              <div class="text-xs text-zinc-600 mb-0.5">Time Spent</div>
              <div class="text-sm font-medium text-white">{{ formatHours(issue.spentHours) }}</div>
            </div>
            <div class="bg-zinc-900 rounded-lg p-3 border border-white/5">
              <div class="text-xs text-zinc-600 mb-0.5">Estimated</div>
              <div class="text-sm font-medium text-white">{{ formatHours(issue.estimatedHours) }}</div>
            </div>
          </div>

          <!-- Description -->
          <div>
            <div class="text-xs font-semibold text-zinc-600 uppercase tracking-wider mb-2">Description</div>
            <div class="text-sm whitespace-pre-wrap bg-zinc-900 border border-white/5 rounded-lg p-3 text-zinc-300">
              {{ issue.description || 'No description provided.' }}
            </div>
          </div>

          <!-- History & Comments -->
          <div v-if="issue.journals && issue.journals.length > 0">
            <div class="text-xs font-semibold text-zinc-600 uppercase tracking-wider mb-3">History</div>
            <div class="relative pl-5">
              <div class="absolute left-1.5 top-2 bottom-2 w-px bg-zinc-800" />
              <div class="space-y-4">
                <div v-for="j in issue.journals" :key="j.id" class="relative">
                  <div class="absolute -left-5 top-1 w-3 h-3 rounded-full border-2 border-zinc-500 bg-zinc-950" />
                  <div class="ml-2">
                    <div class="flex items-center gap-2 mb-1">
                      <span class="text-sm font-medium text-white">{{ j.userName }}</span>
                      <span class="text-xs text-zinc-600">{{ formatDate(j.createdOn) }}</span>
                    </div>
                    <div v-for="d in j.details" :key="d.name" class="text-xs text-zinc-500 mb-0.5">
                      <span class="text-zinc-400">{{ formatPropName(d.name) }}</span>
                      <span v-if="d.oldValue"> changed from <em class="text-zinc-300">{{ d.oldValue }}</em> to <em class="text-zinc-300">{{ d.newValue }}</em></span>
                      <span v-else> set to <em class="text-zinc-300">{{ d.newValue }}</em></span>
                    </div>
                    <div v-if="j.notes" class="text-sm bg-zinc-900 border border-white/5 rounded-lg p-2 mt-1 whitespace-pre-wrap text-zinc-300">
                      {{ j.notes }}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Comment input -->
        <div class="border-t border-white/10 p-4">
          <div v-if="commentError" class="text-sm text-zinc-400 mb-2">{{ commentError }}</div>
          <div class="flex gap-2">
            <textarea
              v-model="commentText"
              placeholder="Add a comment..."
              rows="2"
              class="flex-1 px-3 py-2 border border-white/10 rounded-lg bg-zinc-900 text-white text-sm resize-none focus:outline-none focus:ring-1 focus:ring-white/20 placeholder-zinc-600"
            />
            <button
              @click="submitComment"
              :disabled="submittingComment || !commentText.trim()"
              class="px-4 py-2 bg-white hover:bg-zinc-200 disabled:opacity-30 text-black rounded-lg text-sm font-medium self-center transition-colors"
            >
              {{ submittingComment ? '...' : 'Send' }}
            </button>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>
