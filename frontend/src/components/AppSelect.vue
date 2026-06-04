<script setup lang="ts" generic="T">
import { Listbox, ListboxButton, ListboxOptions, ListboxOption } from '@headlessui/vue'

defineProps<{
  modelValue: T
  options: { value: T; label: string }[]
  placeholder?: string
  disabled?: boolean
}>()

defineEmits<{ (e: 'update:modelValue', value: T): void }>()
</script>

<template>
  <Listbox :model-value="modelValue" @update:model-value="$emit('update:modelValue', $event)" :disabled="disabled">
    <div class="relative">
      <ListboxButton
        class="flex items-center gap-2 text-sm px-2 py-1.5 border border-white/10 rounded-lg bg-zinc-900 text-white focus:outline-none focus:ring-1 focus:ring-white/30 min-w-[120px] disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
      >
        <span class="flex-1 text-left truncate">
          {{ options.find(o => o.value === modelValue)?.label ?? placeholder ?? '—' }}
        </span>
        <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5 text-zinc-500 flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <path d="m6 9 6 6 6-6"/>
        </svg>
      </ListboxButton>

      <Transition
        leave-active-class="transition duration-100 ease-in"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <ListboxOptions
          class="absolute z-50 mt-1 right-0 min-w-full w-max max-h-60 overflow-y-auto rounded-lg border border-white/10 bg-zinc-900 shadow-xl focus:outline-none py-1"
        >
          <ListboxOption
            v-for="option in options"
            :key="String(option.value)"
            :value="option.value"
            v-slot="{ active, selected }"
          >
            <li
              class="flex items-center gap-2 px-3 py-1.5 text-sm cursor-pointer select-none"
              :class="[
                active ? 'bg-white/10 text-white' : 'text-zinc-300',
                selected ? 'font-medium text-white' : ''
              ]"
            >
              <span class="w-3 flex-shrink-0">
                <svg v-if="selected" xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M20 6 9 17l-5-5"/>
                </svg>
              </span>
              {{ option.label }}
            </li>
          </ListboxOption>
        </ListboxOptions>
      </Transition>
    </div>
  </Listbox>
</template>
