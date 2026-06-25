<template>
  <div class="login-page">
    <div class="login-bg-pattern"></div>
    <v-container class="fill-height login-container">
      <v-row justify="center" align="center">
        <v-col cols="12" sm="8" md="4" lg="3">
          <v-card class="login-card" elevation="0">
            <!-- Logo -->
            <div class="login-header">
              <v-img src="@/assets/logo.svg" :width="56" :height="56" class="login-logo" />
              <h2 class="login-brand">1S-UI</h2>
              <p class="login-subtitle">{{ $t('login.title') }}</p>
            </div>

            <!-- Form -->
            <v-card-text class="login-form">
              <v-form @submit.prevent="login" ref="form">
                <v-text-field
                  v-model="username"
                  :label="$t('login.username')"
                  :rules="usernameRules"
                  required
                  prepend-inner-icon="mdi-account-outline"
                  class="login-input"
                />
                <v-text-field
                  v-model="password"
                  :label="$t('login.password')"
                  :rules="passwordRules"
                  type="password"
                  required
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

              <!-- Settings Row -->
              <div class="login-settings">
                <v-select
                  density="compact"
                  hide-details
                  variant="solo"
                  :items="languages"
                  v-model="$i18n.locale"
                  @update:modelValue="changeLocale"
                  class="lang-select"
                />
                <v-menu location="bottom end">
                  <template v-slot:activator="{ props }">
                    <v-btn icon variant="text" size="small" v-bind="props">
                      <v-icon icon="mdi-palette-outline" size="20" />
                    </v-btn>
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
        </v-col>
      </v-row>
    </v-container>
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
  { value: 'cyberpunk', icon: 'mdi-robot' },
  { value: 'nord', icon: 'mdi-snowflake' },
  { value: 'dracula', icon: 'mdi-bat' },
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
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  position: relative;
  overflow: hidden;
}

.login-bg-pattern {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(circle at 20% 50%, rgba(var(--v-theme-primary), 0.06) 0%, transparent 50%),
    radial-gradient(circle at 80% 20%, rgba(var(--v-theme-secondary), 0.04) 0%, transparent 50%),
    radial-gradient(circle at 50% 80%, rgba(var(--v-theme-primary), 0.03) 0%, transparent 50%);
  pointer-events: none;
}

.login-container {
  position: relative;
  z-index: 1;
}

.login-card {
  border-radius: 20px !important;
  padding: 0;
  backdrop-filter: blur(16px) saturate(180%);
  -webkit-backdrop-filter: blur(16px) saturate(180%);
  background: rgba(var(--v-theme-surface), 0.55) !important;
  border: 1px solid rgba(255, 255, 255, 0.12) !important;
  box-shadow:
    0 8px 32px rgba(0, 0, 0, 0.08),
    0 2px 8px rgba(0, 0, 0, 0.04),
    inset 0 1px 0 rgba(255, 255, 255, 0.12),
    inset 0 -1px 0 rgba(0, 0, 0, 0.02) !important;
  overflow: hidden;
  position: relative;
  z-index: 1;
}

.login-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 32px 24px 16px;
  gap: 8px;
}

.login-logo {
  filter: drop-shadow(0 2px 8px rgba(var(--v-theme-primary), 0.2));
}

.login-brand {
  font-size: 22px;
  font-weight: 700;
  letter-spacing: 0.5px;
  margin: 0;
  background: linear-gradient(135deg, rgb(var(--v-theme-primary)), rgb(var(--v-theme-secondary)));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.login-subtitle {
  font-size: 13px;
  opacity: 0.5;
  margin: 0;
}

.login-form {
  padding: 8px 24px 24px !important;
}

.login-input {
  margin-bottom: 4px;
}

.login-btn {
  margin-top: 12px;
  font-size: 15px;
  font-weight: 600;
  letter-spacing: 0.02em;
  height: 44px !important;
  border-radius: 12px !important;
}

.login-settings {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 16px;
}

.lang-select {
  flex: 1;
}

.theme-card {
  border-radius: 14px !important;
  overflow: hidden;
  min-width: 220px;
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
  padding: 7px 10px;
  border-radius: 8px;
  border: 1px solid transparent;
  background: transparent;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  color: rgb(var(--v-theme-on-surface));
  transition: all 0.2s ease;
  font-family: inherit;
}

.theme-chip:hover {
  background: rgba(var(--v-theme-primary), 0.08);
}

.theme-chip--active {
  background: rgba(var(--v-theme-primary), 0.12);
  border-color: rgba(var(--v-theme-primary), 0.3);
  color: rgb(var(--v-theme-primary));
}
</style>