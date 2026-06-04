<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useConfigStore } from '../stores/config'
import { api } from '../lib/api'
import type { StatusItem } from '../lib/types'
import AppSelect from '../components/AppSelect.vue'

const config = useConfigStore()
const router = useRouter()

// Phase: 1 = connection, 2 = columns, 3 = card style
const phase = ref(config.isConnected && config.hasColumnConfig ? 3 : config.isConnected ? 2 : 1)

// Phase 1
const urlInput = ref(config.redmineUrl)
const keyInput = ref(config.apiKey)
const projectInput = ref(config.selectedProject)
const p1Loading = ref(false)
const p1Error = ref('')
const availableProjects = ref<{ id: number; name: string; identifier: string }[]>([])
const projectsLoaded = ref(false)

// Phase 2
const allStatuses = ref<StatusItem[]>([])
const selectedIds = ref<number[]>([...config.columnOrder])
const p2Loading = ref(false)
const p2Error = ref('')

// Phase 3
const colorInput = ref(config.cardColor)
const colorError = ref('')

onMounted(async () => {
  if (config.isConnected) {
    // Pre-load projects so the dropdown is populated when returning to setup
    localStorage.setItem('vermilion_redmine_url', config.redmineUrl)
    localStorage.setItem('vermilion_api_key', config.apiKey)
    try {
      availableProjects.value = await api.listProjects()
      projectsLoaded.value = true
    } catch {}
  }
  if (phase.value >= 2 && config.isConnected) {
    await loadStatuses()
  }
})

async function loadStatuses() {
  p2Loading.value = true
  p2Error.value = ''
  try {
    allStatuses.value = await api.listStatuses()
    if (selectedIds.value.length === 0) {
      selectedIds.value = allStatuses.value.map(s => s.id)
    }
  } catch (e: any) {
    p2Error.value = e.message || 'Failed to load statuses'
  } finally {
    p2Loading.value = false
  }
}

async function fetchProjects() {
  if (!urlInput.value.trim() || !keyInput.value.trim()) {
    p1Error.value = 'URL and API key are required'
    return
  }
  p1Loading.value = true
  p1Error.value = ''
  localStorage.setItem('vermilion_redmine_url', urlInput.value.trim())
  localStorage.setItem('vermilion_api_key', keyInput.value.trim())
  try {
    availableProjects.value = await api.listProjects()
    projectsLoaded.value = true
    if (!projectInput.value && availableProjects.value.length > 0) {
      projectInput.value = availableProjects.value[0].identifier
    }
  } catch (e: any) {
    p1Error.value = e.message || 'Cannot connect to Redmine'
    projectsLoaded.value = false
  } finally {
    p1Loading.value = false
  }
}

async function connectAndNext() {
  if (!projectInput.value) {
    p1Error.value = 'Select a project'
    return
  }
  p1Loading.value = true
  p1Error.value = ''
  try {
    config.saveConnection(urlInput.value.trim(), keyInput.value.trim(), projectInput.value)
    await loadStatuses()
    phase.value = 2
  } catch (e: any) {
    p1Error.value = e.message || 'Failed to load statuses'
  } finally {
    p1Loading.value = false
  }
}

function toggleStatus(id: number) {
  const idx = selectedIds.value.indexOf(id)
  if (idx >= 0) {
    if (selectedIds.value.length <= 1) return // keep at least 1
    selectedIds.value.splice(idx, 1)
  } else {
    selectedIds.value.push(id)
  }
}

function moveUp(id: number) {
  const idx = selectedIds.value.indexOf(id)
  if (idx > 0) {
    const arr = [...selectedIds.value]
    ;[arr[idx - 1], arr[idx]] = [arr[idx], arr[idx - 1]]
    selectedIds.value = arr
  }
}

function moveDown(id: number) {
  const idx = selectedIds.value.indexOf(id)
  if (idx < selectedIds.value.length - 1) {
    const arr = [...selectedIds.value]
    ;[arr[idx], arr[idx + 1]] = [arr[idx + 1], arr[idx]]
    selectedIds.value = arr
  }
}

function saveColumns() {
  config.saveColumnOrder([...selectedIds.value])
  phase.value = 3
}

function isValidColor(hex: string) {
  return /^#[0-9A-Fa-f]{6}$/.test(hex)
}

function saveColorAndFinish() {
  if (!isValidColor(colorInput.value)) {
    colorError.value = 'Enter a valid hex color (#RRGGBB)'
    return
  }
  config.saveCardColor(colorInput.value)
  router.push('/board')
}

function resetColor() {
  colorInput.value = '#C7ECFF'
  colorError.value = ''
}

// YIQ contrast for preview
function textColor(hex: string) {
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return (r * 299 + g * 587 + b * 114) / 1000 >= 128 ? '#1a1a1a' : '#ffffff'
}
</script>

<template>
  <div class="min-h-screen bg-black flex items-center justify-center p-4">
    <div class="w-full max-w-md">

      <!-- Header -->
      <div class="text-center mb-8">
        <h1 class="text-3xl font-bold text-white tracking-tight">Vermilion</h1>
        <p class="text-zinc-600 mt-1">Redmine Kanban Board</p>
      </div>

      <!-- Step indicators -->
      <div class="flex items-center justify-center gap-2 mb-6">
        <div v-for="n in 3" :key="n" class="flex items-center">
          <div
            class="w-8 h-8 rounded-full flex items-center justify-center text-sm font-semibold transition-colors"
            :class="[
              phase >= n ? 'bg-white text-black' : 'bg-zinc-900 text-zinc-600 border border-white/10',
              n < phase ? 'cursor-pointer hover:bg-zinc-200' : ''
            ]"
            @click="n < phase ? phase = n : null"
          >{{ n }}</div>
          <div v-if="n < 3" class="w-8 h-0.5 mx-1"
            :class="phase > n ? 'bg-white' : 'bg-zinc-800'"
          />
        </div>
      </div>

      <!-- Phase 1: Connection -->
      <div v-if="phase === 1" class="bg-zinc-950 border border-white/10 rounded-xl p-6">
        <h2 class="text-lg font-semibold text-white mb-4">Connect to Redmine</h2>
        <div class="space-y-4">
          <div>
            <label class="block text-xs font-medium text-zinc-500 uppercase tracking-wider mb-1">Redmine URL</label>
            <input
              v-model="urlInput"
              type="url"
              placeholder="http://localhost:3000"
              class="w-full px-3 py-2 border border-white/10 rounded-lg bg-zinc-900 text-white placeholder-zinc-600 focus:outline-none focus:ring-1 focus:ring-white/30"
            />
          </div>
          <div>
            <label class="block text-xs font-medium text-zinc-500 uppercase tracking-wider mb-1">API Key</label>
            <input
              v-model="keyInput"
              type="password"
              placeholder="Your Redmine REST API key"
              class="w-full px-3 py-2 border border-white/10 rounded-lg bg-zinc-900 text-white placeholder-zinc-600 focus:outline-none focus:ring-1 focus:ring-white/30"
            />
          </div>
          <div v-if="p1Error" class="text-sm text-zinc-400 bg-zinc-900 border border-white/10 px-3 py-2 rounded-lg">{{ p1Error }}</div>
          <button
            @click="fetchProjects"
            :disabled="p1Loading"
            class="w-full py-2 px-4 bg-zinc-800 hover:bg-zinc-700 disabled:opacity-40 text-white rounded-lg font-medium transition-colors"
          >
            {{ p1Loading && !projectsLoaded ? 'Loading...' : 'Load Projects' }}
          </button>

          <div v-if="projectsLoaded" class="space-y-3">
            <div>
              <label class="block text-xs font-medium text-zinc-500 uppercase tracking-wider mb-1">Project</label>
              <div class="w-full [&>div]:w-full [&_button]:w-full">
                <AppSelect
                  v-model="projectInput"
                  :options="[{ value: '', label: 'Select a project...' }, ...availableProjects.map(p => ({ value: p.identifier, label: p.name }))]"
                />
              </div>
            </div>
            <button
              @click="connectAndNext"
              :disabled="p1Loading || !projectInput"
              class="w-full py-2 px-4 bg-white hover:bg-zinc-200 disabled:opacity-40 text-black rounded-lg font-medium transition-colors inline-flex items-center justify-center gap-2"
            >
              <span>{{ p1Loading ? 'Setting up...' : 'Next' }}</span>
              <svg v-if="!p1Loading" xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <path d="M5 12h14M12 5l7 7-7 7"/>
              </svg>
            </button>
          </div>
        </div>
      </div>

      <!-- Phase 2: Column config -->
      <div v-else-if="phase === 2" class="bg-zinc-950 border border-white/10 rounded-xl p-6">
        <h2 class="text-lg font-semibold text-white mb-1">Configure Columns</h2>
        <p class="text-sm text-zinc-600 mb-4">Choose and order the statuses to show as kanban columns</p>
        <div v-if="p2Loading" class="text-center py-8 text-zinc-600">Loading statuses...</div>
        <div v-else-if="p2Error" class="text-sm text-zinc-400">{{ p2Error }}</div>
        <div v-else class="space-y-2 max-h-72 overflow-y-auto pr-1">
          <div
            v-for="(id, idx) in selectedIds"
            :key="id"
            class="flex items-center gap-2 p-2 bg-zinc-900 border border-white/5 rounded-lg"
          >
            <span class="flex-1 text-sm font-medium text-white">
              {{ allStatuses.find(s => s.id === id)?.name }}
              <span v-if="allStatuses.find(s => s.id === id)?.isClosed" class="text-xs text-zinc-600 ml-1">(closed)</span>
            </span>
            <button @click="moveUp(id)" :disabled="idx === 0" class="p-1 disabled:opacity-20 text-zinc-500 hover:text-white">↑</button>
            <button @click="moveDown(id)" :disabled="idx === selectedIds.length - 1" class="p-1 disabled:opacity-20 text-zinc-500 hover:text-white">↓</button>
            <button @click="toggleStatus(id)" class="p-1 text-zinc-600 hover:text-white">✕</button>
          </div>

          <div v-for="s in allStatuses.filter(s => !selectedIds.includes(s.id))" :key="s.id"
            class="flex items-center gap-2 p-2 rounded-lg opacity-30"
          >
            <span class="flex-1 text-sm text-zinc-400">{{ s.name }}</span>
            <button @click="toggleStatus(s.id)" class="p-1 text-zinc-400 hover:text-white text-xs">+ Add</button>
          </div>
        </div>
        <div class="flex gap-2 mt-6">
          <button @click="phase = 1" class="px-4 py-2 border border-white/10 rounded-lg text-sm text-zinc-400 hover:text-white">Back</button>
          <button @click="saveColumns" :disabled="selectedIds.length === 0"
            class="flex-1 py-2 bg-white hover:bg-zinc-200 disabled:opacity-40 text-black rounded-lg font-medium text-sm">
            Save Columns
          </button>
        </div>
      </div>

      <!-- Phase 3: Card color -->
      <div v-else class="bg-zinc-950 border border-white/10 rounded-xl p-6">
        <h2 class="text-lg font-semibold text-white mb-1">Card Highlight Color</h2>
        <p class="text-sm text-zinc-600 mb-4">Cards assigned to you will be highlighted in this color</p>
        <div class="space-y-4">
          <div class="flex items-center gap-3">
            <input type="color" v-model="colorInput"
              class="w-12 h-10 rounded cursor-pointer border border-white/10 bg-zinc-900" />
            <input v-model="colorInput" type="text" maxlength="7" placeholder="#C7ECFF"
              class="flex-1 px-3 py-2 border border-white/10 rounded-lg bg-zinc-900 text-white font-mono text-sm focus:outline-none focus:ring-1 focus:ring-white/30 uppercase placeholder-zinc-600"
              @input="colorInput = colorInput.toUpperCase()" />
            <button @click="resetColor" class="px-3 py-2 text-sm border border-white/10 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors">Reset</button>
          </div>
          <!-- Preview card -->
          <div v-if="isValidColor(colorInput)"
            class="p-3 rounded-lg border text-sm"
            :style="{ backgroundColor: colorInput, color: textColor(colorInput), borderColor: 'transparent' }"
          >
            <div class="font-semibold">#42 Example task assigned to you</div>
            <div class="flex justify-between mt-1 text-xs opacity-70">
              <span>You</span>
              <span>Today</span>
            </div>
          </div>
          <div v-if="colorError" class="text-sm text-zinc-400">{{ colorError }}</div>
          <div class="flex gap-2">
            <button @click="phase = 2" class="px-4 py-2 border border-white/10 rounded-lg text-sm text-zinc-400 hover:text-white">Back</button>
            <button @click="saveColorAndFinish"
              class="flex-1 py-2 bg-white hover:bg-zinc-200 text-black rounded-lg font-medium text-sm transition-colors">
              Start using Vermilion
            </button>
          </div>
        </div>
      </div>

    </div>
  </div>
</template>
