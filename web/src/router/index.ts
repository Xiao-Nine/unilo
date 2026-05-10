import { createRouter, createWebHistory } from 'vue-router'
import SetupView from '../views/SetupView.vue'
import AuthView from '../views/AuthView.vue'
import AppShell from '../views/AppShell.vue'
import { setUnauthorizedHandler } from '../api/client'
import { useSessionStore } from '../stores/session'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/app' },
    { path: '/setup', component: SetupView },
    { path: '/login', component: AuthView },
    { path: '/app', component: AppShell },
    { path: '/:pathMatch(.*)*', redirect: '/app' },
  ],
})

setUnauthorizedHandler(() => {
  const session = useSessionStore()
  session.expireAuth()
  if (router.currentRoute.value.path !== '/login') {
    void router.push('/login')
  }
})

router.beforeEach(async (to) => {
  const session = useSessionStore()

  if (!session.hasVerifiedServer && to.path !== '/setup') {
    return '/setup'
  }

  if (session.hasVerifiedServer && !session.accessToken && to.path !== '/login') {
    return '/login'
  }

  if (session.accessToken && !session.user) {
    await session.loadMe()
    if (!session.accessToken && to.path !== '/login') {
      return '/login'
    }
  }

  if (session.hasVerifiedServer && !session.accessToken && to.path !== '/login') {
    return '/login'
  }

  if (to.path === '/setup' && session.hasVerifiedServer) {
    return session.isAuthenticated ? '/app' : '/login'
  }

  if (to.path === '/login' && session.isAuthenticated) {
    return '/app'
  }

  return true
})
