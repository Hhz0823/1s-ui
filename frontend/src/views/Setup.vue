<template>
  <div class="setup-page">
    <div class="setup-bg-pattern" />
    <main class="setup-shell">
      <v-card class="setup-card" elevation="0">
        <div class="setup-header">
          <v-img src="@/assets/logo.svg" :width="64" :height="64" class="setup-logo" />
          <h1 class="setup-brand">1S-UI</h1>
          <p class="setup-title">{{ $t('setup.title') }}</p>
        </div>

        <v-card-text class="setup-form">
          <v-form ref="form" @submit.prevent="submit">
            <v-text-field
              v-model="username"
              :label="$t('setup.username')"
              :rules="usernameRules"
              autocomplete="username"
              density="comfortable"
              variant="outlined"
              prepend-inner-icon="mdi-account-outline"
              class="setup-input"
            />
            <v-text-field
              v-model="password"
              :label="$t('setup.password')"
              :rules="passwordRules"
              :type="showPassword ? 'text' : 'password'"
              autocomplete="new-password"
              density="comfortable"
              variant="outlined"
              prepend-inner-icon="mdi-lock-outline"
              :append-inner-icon="showPassword ? 'mdi-eye-off-outline' : 'mdi-eye-outline'"
              class="setup-input"
              @click:append-inner="showPassword = !showPassword"
            />
            <v-text-field
              v-model="confirmPassword"
              :label="$t('setup.confirmPassword')"
              :rules="confirmRules"
              :type="showPassword ? 'text' : 'password'"
              autocomplete="new-password"
              density="comfortable"
              variant="outlined"
              prepend-inner-icon="mdi-lock-check-outline"
              class="setup-input"
            />
            <v-btn
              :loading="loading"
              :disabled="checking"
              type="submit"
              color="primary"
              block
              size="large"
              class="setup-button"
            >
              {{ $t('setup.createAdmin') }}
            </v-btn>
          </v-form>

          <div class="setup-footer">
            <v-select
              v-model="$i18n.locale"
              :items="languages"
              density="comfortable"
              hide-details
              variant="outlined"
              class="language-select"
              @update:model-value="changeLocale"
            />
          </div>
        </v-card-text>
      </v-card>
    </main>
  </div>
</template>

<script lang="ts" setup>
import { onMounted, ref } from 'vue'
import { useLocale } from 'vuetify'
import { useRouter } from 'vue-router'
import { i18n, languages } from '@/locales'
import HttpUtil from '@/plugins/httputil'

const router = useRouter()
const locale = useLocale()
const form = ref<any>(null)
const username = ref('')
const password = ref('')
const confirmPassword = ref('')
const showPassword = ref(false)
const loading = ref(false)
const checking = ref(true)

const charCount = (value: string) => Array.from(value ?? '').length
const usernameRules = [
  (value: string) => charCount(value?.trim()) >= 3 || i18n.global.t('setup.usernameRule'),
  (value: string) => charCount(value?.trim()) <= 64 || i18n.global.t('setup.usernameRule'),
]
const passwordRules = [
  (value: string) => charCount(value) >= 8 || i18n.global.t('setup.passwordRule'),
  (value: string) => charCount(value) <= 128 || i18n.global.t('setup.passwordRule'),
]
const confirmRules = [
  (value: string) => value === password.value || i18n.global.t('setup.confirmRule'),
]

const submit = async () => {
  const result = await form.value?.validate()
  if (!result?.valid || loading.value) return
  loading.value = true
  const response = await HttpUtil.post('api/setup', {
    username: username.value.trim(),
    password: password.value,
    confirmPassword: confirmPassword.value,
  })
  loading.value = false
  if (response.success) await router.replace('/')
}

const changeLocale = (value: any) => {
  locale.current.value = value ?? 'zhHans'
  localStorage.setItem('locale', locale.current.value)
}

onMounted(async () => {
  const response = await HttpUtil.get('api/setup-status')
  checking.value = false
  if (response.success && response.obj?.required === false) {
    await router.replace('/login')
  }
})
</script>

<style scoped>
.setup-page {
  min-height: 100vh;
  min-height: 100dvh;
  display: grid;
  place-items: center;
  position: relative;
  overflow: auto;
  padding: 24px;
  background: rgb(var(--v-theme-background));
}

.setup-bg-pattern {
  position: fixed;
  inset: 0;
  pointer-events: none;
  background:
    linear-gradient(180deg, rgba(var(--v-theme-primary), 0.05), transparent 38%),
    linear-gradient(90deg, rgba(var(--v-theme-on-surface), 0.035) 1px, transparent 1px),
    linear-gradient(180deg, rgba(var(--v-theme-on-surface), 0.035) 1px, transparent 1px);
  background-size: auto, 44px 44px, 44px 44px;
}

.setup-shell {
  position: relative;
  z-index: 1;
  width: min(100%, 420px);
}

.setup-card {
  overflow: hidden;
  padding: 32px 34px 28px;
  border: 1px solid rgba(var(--v-theme-on-surface), 0.08) !important;
  border-radius: 8px !important;
  background: rgba(var(--v-theme-surface), 0.96) !important;
  box-shadow: 0 18px 50px rgba(0, 0, 0, 0.08), 0 2px 8px rgba(0, 0, 0, 0.04) !important;
}

.setup-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding-bottom: 24px;
  text-align: center;
}

.setup-logo { flex: 0 0 auto; }
.setup-brand { margin: 0; font-size: 26px; line-height: 1.2; letter-spacing: 0; }
.setup-title { margin: 0; color: rgba(var(--v-theme-on-surface), 0.68); font-size: 14px; }
.setup-form { padding: 0 !important; }
.setup-input { margin-bottom: 4px; }
.setup-button {
  display: flex !important;
  width: 100% !important;
  max-width: none !important;
  min-height: 44px;
  margin-top: 4px;
  border-radius: 6px !important;
  letter-spacing: 0;
}
.setup-footer { display: flex; justify-content: center; margin-top: 18px; }
.language-select { max-width: 220px; }

@media (max-width: 520px) {
  .setup-page { padding: 16px; }
  .setup-card { padding: 26px 22px 22px; }
}
</style>
