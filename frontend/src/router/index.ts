import { createRouter, createWebHistory } from 'vue-router'
import SetupView from '../views/SetupView.vue'
import BoardView from '../views/BoardView.vue'
import PresetsView from '../views/PresetsView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/setup', component: SetupView },
    { path: '/board', component: BoardView },
    { path: '/presets', component: PresetsView },
    { path: '/', redirect: '/board' },
  ],
})

router.beforeEach((to) => {
  const url = localStorage.getItem('vermilion_redmine_url')
  const key = localStorage.getItem('vermilion_api_key')
  const columns = localStorage.getItem('vermilion_column_order')
  const configured = url && key && columns && JSON.parse(columns).length > 0
  if (!configured && to.path !== '/setup') return '/setup'
})
