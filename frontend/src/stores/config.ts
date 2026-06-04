import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { ProjectItem } from '../lib/types'
import { api } from '../lib/api'

export const useConfigStore = defineStore('config', () => {
  // Connection config
  const redmineUrl = ref(localStorage.getItem('vermilion_redmine_url') || '')
  const apiKey = ref(localStorage.getItem('vermilion_api_key') || '')
  const selectedProject = ref(localStorage.getItem('vermilion_project') || '')
  const projects = ref<ProjectItem[]>([])

  // Column order: array of status IDs in order (saved to localStorage)
  const columnOrder = ref<number[]>(
    JSON.parse(localStorage.getItem('vermilion_column_order') || '[]')
  )

  // Card highlight color
  const cardColor = ref(localStorage.getItem('vermilion_card_color') || '#C7ECFF')

  // Theme
  const theme = ref<'light' | 'dark'>(
    (localStorage.getItem('vermilion_theme') as 'light' | 'dark') || 'light'
  )

  const loading = ref(false)
  const error = ref('')

  const isConnected = computed(() => redmineUrl.value !== '' && apiKey.value !== '')
  const hasColumnConfig = computed(() => columnOrder.value.length > 0)
  const isReady = computed(() => isConnected.value && hasColumnConfig.value)

  function saveConnection(url: string, key: string, project: string) {
    redmineUrl.value = url
    apiKey.value = key
    selectedProject.value = project
    localStorage.setItem('vermilion_redmine_url', url)
    localStorage.setItem('vermilion_api_key', key)
    localStorage.setItem('vermilion_project', project)
  }

  function saveColumnOrder(order: number[]) {
    columnOrder.value = order
    localStorage.setItem('vermilion_column_order', JSON.stringify(order))
  }

  function saveCardColor(color: string) {
    cardColor.value = color
    localStorage.setItem('vermilion_card_color', color)
  }

  function toggleTheme() {
    theme.value = theme.value === 'light' ? 'dark' : 'light'
    localStorage.setItem('vermilion_theme', theme.value)
    if (theme.value === 'dark') {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
  }

  function setProject(identifier: string) {
    selectedProject.value = identifier
    localStorage.setItem('vermilion_project', identifier)
  }

  function reset() {
    redmineUrl.value = ''
    apiKey.value = ''
    selectedProject.value = ''
    projects.value = []
    columnOrder.value = []
    localStorage.removeItem('vermilion_redmine_url')
    localStorage.removeItem('vermilion_api_key')
    localStorage.removeItem('vermilion_project')
    localStorage.removeItem('vermilion_column_order')
  }

  async function fetchProjects() {
    loading.value = true
    error.value = ''
    try {
      projects.value = await api.listProjects()
    } catch (e: any) {
      error.value = e.message || 'Failed to fetch projects'
      throw e
    } finally {
      loading.value = false
    }
  }

  return {
    redmineUrl, apiKey, selectedProject, projects,
    columnOrder, cardColor, theme,
    loading, error,
    isConnected, hasColumnConfig, isReady,
    saveConnection, saveColumnOrder, saveCardColor,
    toggleTheme, setProject, reset, fetchProjects,
  }
})
