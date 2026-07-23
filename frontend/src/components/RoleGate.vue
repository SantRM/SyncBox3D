<script setup lang="ts">
import type { Role } from '@/services/types'
import { useAuthStore } from '@/stores/auth'

interface Props {
  // Roles permitidos. Si faltan, se renderiza si está autenticado.
  roles?: Role[]
}
const props = defineProps<Props>()

const auth = useAuthStore()

function allowed(): boolean {
  if (!auth.isAuthenticated) return false
  if (!props.roles || props.roles.length === 0) return true
  return auth.hasRole(...props.roles)
}
</script>

<template>
  <template v-if="allowed()">
    <slot />
  </template>
</template>
