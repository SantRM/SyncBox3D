import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import type { Role } from '@/services/types'

declare module 'vue-router' {
  interface RouteMeta {
    public?: boolean
    roles?: Role[]
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginView.vue'),
    meta: { public: true },
  },
  {
    path: '/',
    name: 'dashboard',
    component: () => import('@/views/DashboardView.vue'),
  },
  {
    path: '/equipos',
    name: 'equipos',
    component: () => import('@/views/EquiposListView.vue'),
  },
  {
    // IMPORTANTE: declarar ANTES que /equipos/:id para que "nuevo" no se interprete como id.
    path: '/equipos/nuevo',
    name: 'equipo-nuevo',
    component: () => import('@/views/EquipoFormView.vue'),
    meta: { roles: ['ADMINISTRADOR'] },
  },
  {
    path: '/equipos/:id',
    name: 'equipo-detalle',
    component: () => import('@/views/EquipoDetailView.vue'),
    props: true,
  },
  {
    path: '/laboratorios',
    redirect: { name: 'ubicaciones' },
  },
  {
    path: '/laboratorios/:id',
    name: 'laboratorio-detalle',
    component: () => import('@/views/LaboratorioDetailView.vue'),
    props: true,
  },
  {
    path: '/laboratorios/:id/auditoria',
    name: 'laboratorio-auditoria',
    component: () => import('@/views/LaboratorioAuditView.vue'),
    props: true,
  },
  {
    path: '/usuarios',
    name: 'usuarios',
    component: () => import('@/views/UsuariosView.vue'),
    meta: { roles: ['ADMINISTRADOR'] },
  },
  {
    path: '/categorias',
    name: 'categorias',
    component: () => import('@/views/CategoriasView.vue'),
    meta: { roles: ['ADMINISTRADOR'] },
  },
  {
    path: '/ubicaciones',
    name: 'ubicaciones',
    component: () => import('@/views/UbicacionesView.vue'),
    meta: { roles: ['ADMINISTRADOR'] },
  },
  {
    path: '/alertas',
    name: 'alertas',
    component: () => import('@/views/AlertasView.vue'),
    meta: { roles: ['ADMINISTRADOR', 'OPERADOR'] },
  },
  {
    path: '/perfil',
    name: 'perfil',
    component: () => import('@/views/PerfilView.vue'),
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (to.meta.public) {
    if (auth.isAuthenticated && to.name === 'login') return { name: 'dashboard' }
    return true
  }
  if (!auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.roles && !auth.hasRole(...to.meta.roles)) {
    return { name: 'dashboard' }
  }
  return true
})
