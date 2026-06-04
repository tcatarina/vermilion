<script setup lang="ts">
import { RouterView } from 'vue-router'

// Always dark — force the class on mount
document.documentElement.classList.add('dark')
</script>

<template>
  <div class="min-h-screen bg-black text-white">
    <RouterView />
  </div>
</template>
