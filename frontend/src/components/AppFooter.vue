<script setup lang="ts">
import logoSyncbox from '@/assets/logo-syncbox.png'

const year = new Date().getFullYear()
const logoSrc = logoSyncbox
</script>

<template>
  <footer class="footer">
    <div class="footer__inner">
      <img class="footer__logo" :src="logoSrc" :alt="$t('app.name')" />
      <span class="footer__sep">·</span>
      <span>© {{ year }}</span>
      <span class="footer__sep">·</span>
      <span>{{ $t('footer.description') }}</span>
    </div>
  </footer>
</template>

<style scoped>
.footer {
  margin-top: auto;
  background: var(--c-surface);
  border-top: 1px solid var(--c-border);
  color: var(--c-text-muted);
  font-size: 0.82rem;
}
.footer__inner {
  max-width: 1280px;
  margin: 0 auto;
  padding: 0.85rem 1.25rem;
  display: flex; flex-wrap: wrap; align-items: center; gap: 0.5rem;
  justify-content: center;
  text-align: center;
}
.footer__logo {
  display: block;
  width: 96px;
  height: auto;
  object-fit: contain;
  background: #fff;
  border-radius: 3px;
}
.footer__sep { opacity: 0.5; }
</style>
