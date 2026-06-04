import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Preset } from '../lib/types'

export const usePresetsStore = defineStore('presets', () => {
  const presets = ref<Preset[]>(
    JSON.parse(localStorage.getItem('vermilion_presets') || '[]')
  )
  const activePresetId = ref<string | null>(
    localStorage.getItem('vermilion_active_preset') || null
  )

  const activePreset = computed(() =>
    presets.value.find(p => p.id === activePresetId.value) ?? null
  )

  function persist() {
    localStorage.setItem('vermilion_presets', JSON.stringify(presets.value))
  }

  function savePreset(data: Omit<Preset, 'id'>) {
    const id = crypto.randomUUID()
    presets.value.push({ id, ...data })
    persist()
    return id
  }

  function updatePreset(id: string, changes: Partial<Omit<Preset, 'id'>>) {
    const idx = presets.value.findIndex(p => p.id === id)
    if (idx >= 0) {
      presets.value[idx] = { ...presets.value[idx], ...changes }
      persist()
    }
  }

  function deletePreset(id: string) {
    presets.value = presets.value.filter(p => p.id !== id)
    if (activePresetId.value === id) clearPreset()
    else persist()
  }

  function applyPreset(id: string) {
    activePresetId.value = id
    localStorage.setItem('vermilion_active_preset', id)
  }

  function clearPreset() {
    activePresetId.value = null
    localStorage.removeItem('vermilion_active_preset')
  }

  return {
    presets, activePresetId, activePreset,
    savePreset, updatePreset, deletePreset,
    applyPreset, clearPreset,
  }
})
