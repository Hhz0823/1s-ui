<template>
  <div class="login-page">
    <div class="login-bg-pattern"></div>
    <main class="login-shell">
      <v-card class="login-card" elevation="0">
        <div class="login-header">
          <v-img src="@/assets/logo.svg" :width="64" :height="64" class="login-logo" />
          <h1 class="login-brand">1S-UI</h1>
          <p class="login-subtitle">{{ $t('login.title') }}</p>
        </div>

        <v-card-text class="login-form">
          <v-form @submit.prevent="login" ref="form">
            <v-text-field
              v-model="username"
              :label="$t('login.username')"
              :rules="usernameRules"
              required
              autocomplete="username"
              density="comfortable"
              variant="outlined"
              prepend-inner-icon="mdi-account-outline"
              class="login-input"
            />
            <v-text-field
              v-model="password"
              :label="$t('login.password')"
              :rules="passwordRules"
              type="password"
              required
              autocomplete="current-password"
              density="comfortable"
              variant="outlined"
              prepend-inner-icon="mdi-lock-outline"
              class="login-input"
            />
            <v-btn
              :loading="loading"
              type="submit"
              color="primary"
              block
              size="large"
              class="login-btn"
            >
              {{ $t('actions.submit') }}
            </v-btn>
          </v-form>

          <div class="login-settings">
            <v-select
              density="comfortable"
              hide-details
              variant="outlined"
              :items="languages"
              v-model="$i18n.locale"
              @update:modelValue="changeLocale"
              class="lang-select"
            />
            <v-menu location="bottom end">
              <template v-slot:activator="{ props }">
                <v-btn
                  icon="mdi-palette-outline"
                  variant="outlined"
                  color="primary"
                  class="theme-trigger"
                  v-bind="props"
                />
              </template>
              <v-card class="theme-card" elevation="8">
                <div class="theme-grid">
                  <button
                    v-for="th in themes"
                    :key="th.value"
                    @click="changeTheme(th.value)"
                    class="theme-chip"
                    :class="{ 'theme-chip--active': isActiveTheme(th.value) }"
                  >
                    <v-icon :icon="th.icon" size="16" />
                    <span>{{ $t('theme.' + th.value) }}</span>
                  </button>
                </div>
              </v-card>
            </v-menu>
          </div>
        </v-card-text>
      </v-card>
    </main>
  </div>
</template>

<script lang="ts" setup>
import { ref } from "vue"
import { useLocale, useTheme } from 'vuetify'
import { i18n, languages } from '@/locales'
import { useRouter } from 'vue-router'
import HttpUtil from '@/plugins/httputil'

const theme = useTheme()
const locale = useLocale()

const themes = [
  { value: 'light', icon: 'mdi-white-balance-sunny' },
  { value: 'dark', icon: 'mdi-moon-waning-crescent' },
  { value: 'midnight', icon: 'mdi-weather-night' },
  { value: 'ocean', icon: 'mdi-waves' },
  { value: 'sunset', icon: 'mdi-weather-sunset' },
  { value: 'forest', icon: 'mdi-pine-tree' },
  { value: 'sakura', icon: 'mdi-flower' },
  { value: 'daylight', icon: 'mdi-weather-sunny-alert' },
  { value: 'mint', icon: 'mdi-leaf' },
  { value: 'cyberpunk', icon: 'mdi-robot' },
  { value: 'nord', icon: 'mdi-snowflake' },
  { value: 'dracula', icon: 'mdi-bat' },
  { value: 'graphite', icon: 'mdi-circle-slice-8' },
  { value: 'system', icon: 'mdi-laptop' },
]

const username = ref('')
const usernameRules = [
  (value: string) => {
    if (value?.length > 0) return true
    return i18n.global.t('login.unRules')
  },
]

const password = ref('')
const passwordRules = [
  (value: string) => {
    if (value?.length > 0) return true
    return i18n.global.t('login.pwRules')
  },
]

const loading = ref(false)
const router = useRouter()

const login = async () => {
  if (username.value == '' || password.value == '') return
  loading.value = true
  const response = await HttpUtil.post('api/login', { user: username.value, pass: password.value })
  if (response.success) {
    setTimeout(() => {
      loading.value = false
      router.push('/')
    }, 500)
  } else {
    loading.value = false
  }
}

const changeLocale = (l: any) => {
  locale.current.value = l ?? 'zhHans'
  localStorage.setItem('locale', locale.current.value)
}
const changeTheme = (th: string) => {
  theme.change(th)
  localStorage.setItem('theme', th)
}
const isActiveTheme = (th: string) => {
  const current = localStorage.getItem('theme') ?? 'system'
  return current == th
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  min-height: 100dvh;
  display: grid;
  place-items: center;
  background: rgb(var(--v-theme-background));
  position: relative;
  overflow: auto;
  padding: 24px;
}

.login-bg-pattern {
  position: fixed;
  inset: 0;
  background:
    linear-gradient(180deg, rgba(var(--v-theme-primary), 0.05), transparent 38%),
    linear-gradient(90deg, rgba(var(--v-theme-on-surface), 0.035) 1px, transparent 1px),
    linear-gradient(180deg, rgba(var(--v-theme-on-surface), 0.035) 1px, transparent 1px);
  background-size: auto, 44px 44px, 44px 44px;
  pointer-events: none;
}

.login-shell {
  position: relative;
  z-index: 1;
  width: min(100%, 420px);
}

.login-card {
  border-radius: 8px !important;
  padding: 32px 34px 28px;
  background: rgba(var(--v-theme-surface), 0.96) !important;
  border: 1px solid rgba(var(--v-theme-on-surface), 0.08) !important;
  box-shadow:
    0 18px 50px rgba(0, 0, 0, 0.08),
    0 2px 8px rgba(0, 0, 0, 0.04) !important;
  overflow: hidden;
}

.login-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 0 0 24px;
  gap: 10px;
}

.login-logo {
  filter: drop-shadow(0 2px 8px rgba(var(--v-theme-primary), 0.2));
}

.login-brand {
  font-size: 28px;
  line-height: 1;
  font-weight: 700;
  letter-spacing: 0;
  margin: 0;
  background: linear-gradient(135deg, rgb(var(--v-theme-primary)), rgb(var(--v-theme-secondary)));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.login-subtitle {
  font-size: 14px;
  opacity: 0.62;
  margin: 0;
}

.login-form {
  padding: 0 !important;
}

.login-input {
  margin-bottom: 14px;
}

.login-btn {
  width: 100%;
  margin-top: 4px;
  font-size: 15px;
  font-weight: 600;
  letter-spacing: 0;
  height: 48px !important;
  border-radius: 8px !important;
}

.login-settings {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 18px;
}

.lang-select {
  flex: 1;
  min-width: 0;
}

.theme-trigger {
  flex: 0 0 48px;
  width: 48px !important;
  height: 48px !important;
  border-radius: 8px !important;
}

.theme-card {
  border-radius: 8px !important;
  overflow: hidden;
  min-width: 240px;
}

.theme-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px;
  padding: 8px;
}

.theme-chip {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 10px;
  border-radius: 8px;
  border: 1px solid transparent;
  background: transparent;
  background-clip: padding-box;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  color: rgb(var(--v-theme-on-surface));
  transition: all 0.2s ease;
  font-family: inherit;
  isolation: isolate;
  overflow: hidden;
  clip-path: inset(0 round 8px);
}

.theme-chip:hover {
  background: rgba(var(--v-theme-primary), 0.08);
}

.theme-chip--active {
  background: rgba(var(--v-theme-primary), 0.12);
  border-color: rgba(var(--v-theme-primary), 0.3);
  color: rgb(var(--v-theme-primary));
}

.login-card :deep(.v-text-field),
.login-card :deep(.v-select) {
  background: transparent !important;
  border: 0 !important;
  box-shadow: none !important;
}

.login-card :deep(.v-field) {
  min-height: 52px;
  border-radius: 8px !important;
  background: rgba(var(--v-theme-surface), 0.92) !important;
  box-shadow: none !important;
}

.login-card :deep(.v-field__outline) {
  --v-field-border-opacity: 0.16;
}

.login-card :deep(.v-field--focused .v-field__outline) {
  --v-field-border-opacity: 0.42;
}

.login-card :deep(.v-input__details) {
  padding-inline: 2px;
}

@media (max-width: 600px) {
  .login-page {
    padding: 16px;
  }

  .login-shell {
    width: 100%;
  }

  .login-card {
    padding: 28px 22px 22px;
  }
}
</style>
