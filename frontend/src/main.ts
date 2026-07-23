import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import { i18n, setLocale } from './i18n'
import { router } from './router'
import { useAuthStore } from './stores/auth'
import { setOnAuthFailure } from './services/api'
import './styles/main.css'

async function bootstrap(): Promise<void> {
  const app = createApp(App)
  const pinia = createPinia()
  app.use(pinia)
  app.use(i18n)
  setLocale('es')

  const auth = useAuthStore()
  setOnAuthFailure(() => {
    auth.forceLogout()
    void router.push({ name: 'login' })
  })

  // Restaurar sesión si quedó refresh token vivo.
  await auth.bootstrap()

  app.use(router)
  await router.isReady()
  app.mount('#app')
}

void bootstrap()
