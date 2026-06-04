<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePresetsStore } from '../stores/presets'
import { useConfigStore } from '../stores/config'
import AppSelect from '../components/AppSelect.vue'
import { api } from '../lib/api'
import type { Preset, MemberItem, VersionItem, StatusItem } from '../lib/types'

const router = useRouter()
const presetsStore = usePresetsStore()
const config = useConfigStore()

const editingId = ref<string | null>(null)
const formName = ref('')
const formProject = ref('')
const formAssigneeIds = ref<number[]>([])
const formVersionIds = ref<number[]>([])
const formStatusIds = ref<number[]>([])

const formMembers = ref<MemberItem[]>([])
const formVersions = ref<VersionItem[]>([])
const formStatuses = ref<StatusItem[]>([])
const loadingOptions = ref(false)

const showForm = ref(false)

onMounted(async () => {
  if (config.projects.length === 0) await config.fetchProjects()
})

async function loadProjectOptions(identifier: string) {
  if (!identifier) {
    formMembers.value = []
    formVersions.value = []
    formStatuses.value = []
    return
  }
  loadingOptions.value = true
  try {
    const data = await api.loadBoard(identifier)
    formMembers.value = data.members.slice().sort((a, b) => a.name.localeCompare(b.name))
    formVersions.value = data.versions.slice().sort((a, b) => a.name.localeCompare(b.name))
    formStatuses.value = data.statuses.slice().sort((a, b) => a.name.localeCompare(b.name))
  } finally {
    loadingOptions.value = false
  }
}

watch(formProject, (val) => {
  formAssigneeIds.value = []
  formVersionIds.value = []
  formStatusIds.value = []
  loadProjectOptions(val)
})

function openNew() {
  editingId.value = null
  formName.value = ''
  formProject.value = config.selectedProject || ''
  formAssigneeIds.value = []
  formVersionIds.value = []
  formStatusIds.value = []
  showForm.value = true
  if (formProject.value) loadProjectOptions(formProject.value)
}

function openEdit(preset: Preset) {
  editingId.value = preset.id
  formName.value = preset.name
  formProject.value = preset.projectIdentifier
  formAssigneeIds.value = [...preset.assigneeIds]
  formVersionIds.value = [...preset.versionIds]
  formStatusIds.value = [...preset.statusIds]
  showForm.value = true
  loadProjectOptions(preset.projectIdentifier)
}

function cancelForm() {
  showForm.value = false
  editingId.value = null
}

function saveForm() {
  if (!formName.value.trim() || !formProject.value) return
  const data = {
    name: formName.value.trim(),
    projectIdentifier: formProject.value,
    assigneeIds: [...formAssigneeIds.value],
    versionIds: [...formVersionIds.value],
    statusIds: [...formStatusIds.value],
  }
  if (editingId.value) {
    presetsStore.updatePreset(editingId.value, data)
  } else {
    presetsStore.savePreset(data)
  }
  showForm.value = false
  editingId.value = null
}

function toggleId(arr: number[], id: number) {
  const idx = arr.indexOf(id)
  if (idx >= 0) arr.splice(idx, 1)
  else arr.push(id)
}
</script>

<template>
  <div class="flex flex-col h-screen overflow-hidden bg-black text-white">

    <!-- Header -->
    <header class="flex items-center gap-3 px-4 py-2.5 bg-zinc-950 border-b border-white/10 flex-shrink-0">
      <button
        @click="router.push('/board')"
        class="flex items-center gap-1.5 p-1.5 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors text-sm"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <path d="m15 18-6-6 6-6"/>
        </svg>
        Back
      </button>
      <div class="w-px h-4 bg-white/10" />
      <h1 class="text-base font-semibold text-white">Presets</h1>
    </header>

    <!-- Body -->
    <div class="flex flex-1 overflow-hidden min-h-0">

      <!-- Left: preset list -->
      <aside class="w-56 flex-shrink-0 bg-zinc-950 border-r border-white/10 flex flex-col overflow-hidden">
        <div class="p-3 border-b border-white/10">
          <button
            @click="openNew"
            class="w-full px-3 py-1.5 bg-white text-black text-sm font-medium rounded-lg hover:bg-zinc-200 transition-colors"
          >
            + New preset
          </button>
        </div>
        <div class="flex-1 overflow-y-auto py-1">
          <div v-if="presetsStore.presets.length === 0" class="px-4 py-4 text-xs text-zinc-600">
            No presets yet.
          </div>
          <div
            v-for="p in presetsStore.presets"
            :key="p.id"
            class="flex items-center gap-1 px-3 py-2.5 group cursor-pointer hover:bg-zinc-900 transition-colors"
            :class="editingId === p.id ? 'bg-zinc-900' : ''"
            @click="openEdit(p)"
          >
            <span class="flex-1 text-sm truncate text-zinc-200">{{ p.name }}</span>
            <button
              @click.stop="presetsStore.deletePreset(p.id)"
              class="opacity-0 group-hover:opacity-100 text-zinc-600 hover:text-red-400 text-sm px-1 transition-all"
              title="Delete"
            >✕</button>
          </div>
        </div>
      </aside>

      <!-- Right: form or empty state -->
      <main class="flex-1 overflow-y-auto p-6">
        <div v-if="!showForm" class="h-full flex items-center justify-center text-zinc-600 text-sm select-none">
          Select a preset to edit, or create a new one.
        </div>

        <div v-else class="max-w-lg space-y-5">
          <h2 class="text-sm font-semibold text-zinc-400 uppercase tracking-wider">
            {{ editingId ? 'Edit preset' : 'New preset' }}
          </h2>

          <!-- Name -->
          <div>
            <label class="block text-xs font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">Name</label>
            <input
              v-model="formName"
              type="text"
              placeholder="e.g. My sprint filters"
              class="w-full px-3 py-2 bg-zinc-900 border border-white/10 rounded-lg text-sm text-white placeholder-zinc-600 focus:outline-none focus:ring-1 focus:ring-white/20"
            />
          </div>

          <!-- Project -->
          <div>
            <label class="block text-xs font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">Project</label>
            <AppSelect
              :model-value="formProject"
              :options="[{ value: '', label: 'Select project...' }, ...config.projects.slice().sort((a, b) => a.name.localeCompare(b.name)).map(p => ({ value: p.identifier, label: p.name }))]"
              @update:model-value="formProject = $event"
            />
          </div>

          <div v-if="loadingOptions" class="text-xs text-zinc-600 py-2">Loading project data…</div>

          <template v-else-if="formProject">

            <!-- Assignees -->
            <div>
              <label class="block text-xs font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                Assignees
                <span class="text-zinc-700 font-normal normal-case tracking-normal ml-1">(empty = all)</span>
              </label>
              <div v-if="formMembers.length === 0" class="text-xs text-zinc-600">No members found.</div>
              <div v-else class="grid grid-cols-2 gap-0.5 max-h-44 overflow-y-auto pr-1">
                <label
                  v-for="m in formMembers"
                  :key="m.id"
                  class="flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer hover:bg-zinc-900 transition-colors"
                >
                  <input
                    type="checkbox"
                    :checked="formAssigneeIds.includes(m.id)"
                    @change="toggleId(formAssigneeIds, m.id)"
                    class="accent-white flex-shrink-0"
                  />
                  <span class="text-sm text-zinc-300 truncate">{{ m.name }}</span>
                </label>
              </div>
            </div>

            <!-- Versions -->
            <div>
              <label class="block text-xs font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                Versions
                <span class="text-zinc-700 font-normal normal-case tracking-normal ml-1">(empty = all)</span>
              </label>
              <div v-if="formVersions.length === 0" class="text-xs text-zinc-600">No versions found.</div>
              <div v-else class="grid grid-cols-2 gap-0.5 max-h-44 overflow-y-auto pr-1">
                <label
                  v-for="v in formVersions"
                  :key="v.id"
                  class="flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer hover:bg-zinc-900 transition-colors"
                >
                  <input
                    type="checkbox"
                    :checked="formVersionIds.includes(v.id)"
                    @change="toggleId(formVersionIds, v.id)"
                    class="accent-white flex-shrink-0"
                  />
                  <span class="text-sm text-zinc-300 truncate">{{ v.name }}</span>
                </label>
              </div>
            </div>

            <!-- Statuses -->
            <div>
              <label class="block text-xs font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                Columns
                <span class="text-zinc-700 font-normal normal-case tracking-normal ml-1">(empty = all)</span>
              </label>
              <div v-if="formStatuses.length === 0" class="text-xs text-zinc-600">No statuses found.</div>
              <div v-else class="grid grid-cols-2 gap-0.5">
                <label
                  v-for="s in formStatuses"
                  :key="s.id"
                  class="flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer hover:bg-zinc-900 transition-colors"
                >
                  <input
                    type="checkbox"
                    :checked="formStatusIds.includes(s.id)"
                    @change="toggleId(formStatusIds, s.id)"
                    class="accent-white flex-shrink-0"
                  />
                  <span class="text-sm text-zinc-300 truncate">{{ s.name }}</span>
                </label>
              </div>
            </div>

          </template>

          <!-- Actions -->
          <div class="flex gap-2 pt-2 border-t border-white/5">
            <button
              @click="cancelForm"
              class="px-4 py-2 rounded-lg text-sm text-zinc-400 hover:text-white hover:bg-zinc-900 transition-colors"
            >
              Cancel
            </button>
            <button
              @click="saveForm"
              :disabled="!formName.trim() || !formProject"
              class="px-4 py-2 rounded-lg text-sm bg-white text-black font-medium hover:bg-zinc-200 disabled:opacity-30 transition-colors"
            >
              Save preset
            </button>
          </div>
        </div>
      </main>
    </div>
  </div>
</template>
