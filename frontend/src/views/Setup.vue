<template>
  <div class="setup-page">
    <div class="setup-bg-pattern" />
    <main class="setup-shell">
      <v-card class="setup-card" elevation="0">
        <header class="setup-topbar">
          <div class="setup-brand-row">
            <v-img src="@/assets/logo.svg" :width="44" :height="44" class="setup-logo" />
            <div><strong>1S-UI</strong><span>{{ $t('setup.guide') }}</span></div>
          </div>
          <v-btn v-if="step < 2" variant="text" size="small" append-icon="mdi-skip-next-outline" @click="skipTutorial">
            {{ $t('setup.skipGuide') }}
          </v-btn>
        </header>

        <v-progress-linear :model-value="(step + 1) * 100 / 3" height="3" color="primary" />

        <v-card-text class="setup-content">
          <v-window v-model="step" :touch="false">
            <v-window-item :value="0">
              <section class="setup-step">
                <div class="step-heading">
                  <v-icon icon="mdi-hand-wave-outline" color="primary" size="36" />
                  <h1>{{ $t('setup.welcomeTitle') }}</h1>
                  <p>{{ $t('setup.welcomeText') }}</p>
                </div>
                <div class="feature-list">
                  <div v-for="feature in features" :key="feature.title" class="feature-row">
                    <v-icon :icon="feature.icon" :color="feature.color" size="24" />
                    <div><strong>{{ feature.title }}</strong><span>{{ feature.text }}</span></div>
                  </div>
                </div>
              </section>
            </v-window-item>

            <v-window-item :value="1">
              <section class="setup-step">
                <div class="step-heading compact">
                  <v-icon icon="mdi-server-network-outline" color="primary" size="34" />
                  <h1>{{ $t('setup.chooseRole') }}</h1>
                  <p>{{ $t('setup.chooseRoleText') }}</p>
                </div>
                <v-btn-toggle v-model="role" mandatory divided class="role-toggle" color="primary">
                  <v-btn v-for="option in roleOptions" :key="option.value" :value="option.value" :prepend-icon="option.icon">
                    {{ option.title }}
                  </v-btn>
                </v-btn-toggle>
                <div class="role-hint">
                  <v-icon :icon="selectedRole.icon" color="primary" />
                  <span>{{ selectedRole.hint }}</span>
                </div>
              </section>
            </v-window-item>

            <v-window-item :value="2">
              <section class="setup-step final-step">
                <div class="step-heading compact">
                  <v-icon icon="mdi-account-cog-outline" color="primary" size="34" />
                  <h1>{{ $t('setup.finishTitle') }}</h1>
                  <p>{{ $t('setup.finishText') }}</p>
                </div>

                <div class="role-summary">
                  <v-icon :icon="selectedRole.icon" color="primary" />
                  <div><span>{{ $t('setup.currentRole') }}</span><strong>{{ selectedRole.title }}</strong></div>
                  <v-btn v-if="!adminCreated" variant="text" size="small" @click="step = 1">{{ $t('setup.changeRole') }}</v-btn>
                </div>

                <v-alert v-if="postSetupError" type="warning" variant="tonal" density="compact" class="mb-4">
                  {{ postSetupError }}
                </v-alert>

                <v-form ref="form" @submit.prevent="submit">
                  <div class="field-grid">
                    <v-text-field
                      v-model="username"
                      :label="$t('setup.username')"
                      :rules="usernameRules"
                      autocomplete="username"
                      density="comfortable"
                      variant="outlined"
                      prepend-inner-icon="mdi-account-outline"
                      :disabled="adminCreated"
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
                      :disabled="adminCreated"
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
                      :disabled="adminCreated"
                    />
                  </div>

                  <template v-if="role === 'client'">
                    <v-divider class="my-2 mb-5" />
                    <v-text-field
                      v-model="controllerURL"
                      :label="$t('setup.controllerAddress')"
                      :hint="$t('setup.controllerHint')"
                      :rules="controllerRules"
                      persistent-hint
                      density="comfortable"
                      variant="outlined"
                      prepend-inner-icon="mdi-link-variant"
                      dir="ltr"
                    />
                    <v-checkbox
                      v-if="controllerURL.trim()"
                      v-model="insecure"
                      :label="$t('agent.allowInsecure')"
                      :hint="$t('agent.allowInsecureHint')"
                      persistent-hint
                      density="compact"
                    />
                  </template>

                  <div class="final-actions">
                    <v-btn v-if="!adminCreated" variant="text" prepend-icon="mdi-arrow-left" @click="step = 1">{{ $t('setup.back') }}</v-btn>
                    <v-spacer />
                    <v-btn v-if="adminCreated" variant="outlined" @click="enterPanel">{{ $t('setup.enterPanel') }}</v-btn>
                    <v-btn :loading="loading" :disabled="checking" type="submit" color="primary" size="large" prepend-icon="mdi-check-circle-outline">
                      {{ adminCreated ? $t('setup.retrySetup') : $t('setup.completeSetup') }}
                    </v-btn>
                  </div>
                </v-form>
              </section>
            </v-window-item>
          </v-window>
        </v-card-text>

        <footer v-if="step < 2" class="setup-footer">
          <v-btn :disabled="step === 0" variant="text" prepend-icon="mdi-arrow-left" @click="step--">{{ $t('setup.back') }}</v-btn>
          <v-select
            v-model="$i18n.locale"
            :items="languages"
            density="compact"
            hide-details
            variant="outlined"
            class="language-select"
            @update:model-value="changeLocale"
          />
          <v-btn color="primary" append-icon="mdi-arrow-right" @click="step++">{{ $t('setup.next') }}</v-btn>
        </footer>
      </v-card>
    </main>
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue'
import { useLocale } from 'vuetify'
import { useRouter } from 'vue-router'
import { i18n, languages } from '@/locales'
import HttpUtil from '@/plugins/httputil'
import { fetchBackendObject, resolveFrontendUrl } from '@/utils/backend'

type PanelRole = 'client' | 'full' | 'monitor'

const router = useRouter()
const locale = useLocale()
const form = ref<any>(null)
const step = ref(0)
const role = ref<PanelRole>('client')
const username = ref('')
const password = ref('')
const confirmPassword = ref('')
const controllerURL = ref('')
const insecure = ref(false)
const showPassword = ref(false)
const loading = ref(false)
const checking = ref(true)
const adminCreated = ref(false)
const postSetupError = ref('')

const features = computed(() => [
  { icon: 'mdi-view-dashboard-outline', color: 'primary', title: i18n.global.t('setup.featureFleet'), text: i18n.global.t('setup.featureFleetHint') },
  { icon: 'mdi-tune-vertical', color: 'success', title: i18n.global.t('setup.featureControl'), text: i18n.global.t('setup.featureControlHint') },
  { icon: 'mdi-shield-check-outline', color: 'warning', title: i18n.global.t('setup.featureIndependent'), text: i18n.global.t('setup.featureIndependentHint') },
])
const roleOptions = computed(() => [
  { value: 'client' as PanelRole, icon: 'mdi-server-outline', title: i18n.global.t('setting.roleClient'), hint: i18n.global.t('setting.roleClientHint') },
  { value: 'full' as PanelRole, icon: 'mdi-server-network', title: i18n.global.t('setting.roleFull'), hint: i18n.global.t('setting.roleFullHint') },
  { value: 'monitor' as PanelRole, icon: 'mdi-monitor-eye', title: i18n.global.t('setting.roleMonitor'), hint: i18n.global.t('setting.roleMonitorHint') },
])
const selectedRole = computed(() => roleOptions.value.find(option => option.value === role.value) || roleOptions.value[0])

const charCount = (value: string) => Array.from(value ?? '').length
const usernameRules = [
  (value: string) => charCount(value?.trim()) >= 3 || i18n.global.t('setup.usernameRule'),
  (value: string) => charCount(value?.trim()) <= 64 || i18n.global.t('setup.usernameRule'),
]
const passwordRules = [
  (value: string) => charCount(value) >= 8 || i18n.global.t('setup.passwordRule'),
  (value: string) => charCount(value) <= 128 || i18n.global.t('setup.passwordRule'),
]
const confirmRules = [(value: string) => value === password.value || i18n.global.t('setup.confirmRule')]
const controllerRules = [(value: string) => !value?.trim() || /^https?:\/\/[^\s]+$/i.test(value.trim()) || i18n.global.t('setup.controllerRule')]

const skipTutorial = () => { role.value = 'client'; step.value = 2 }
const enterPanel = () => router.replace('/')

const submit = async () => {
  const result = await form.value?.validate()
  if (!result?.valid || loading.value) return
  loading.value = true
  postSetupError.value = ''

  if (!adminCreated.value) {
    const response = await HttpUtil.post('api/setup', {
      username: username.value.trim(),
      password: password.value,
      confirmPassword: confirmPassword.value,
    })
    if (!response.success) {
      loading.value = false
      return
    }
    adminCreated.value = true
  }

  try {
    await fetchBackendObject('api/controller-mode', {
      method: 'POST',
      body: JSON.stringify({ profile: role.value }),
    })
    if (role.value === 'client' && controllerURL.value.trim()) {
      await fetchBackendObject('api/agents/connect-local', {
        method: 'POST',
        body: JSON.stringify({
          connect_url: controllerURL.value.trim(),
          public_url: resolveFrontendUrl(),
          insecure: insecure.value,
        }),
      })
    }
    await router.replace('/')
  } catch (error: any) {
    postSetupError.value = error?.message || i18n.global.t('setup.configureFailed')
  } finally {
    loading.value = false
  }
}

const changeLocale = (value: any) => {
  locale.current.value = value ?? 'zhHans'
  localStorage.setItem('locale', locale.current.value)
}

onMounted(async () => {
  const response = await HttpUtil.get('api/setup-status')
  checking.value = false
  if (response.success && response.obj?.required === false) {
    await router.replace(response.obj?.authenticated ? '/' : '/login')
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
    linear-gradient(180deg, rgba(var(--v-theme-primary), 0.08), transparent 44%),
    linear-gradient(90deg, rgba(var(--v-theme-on-surface), 0.035) 1px, transparent 1px),
    linear-gradient(180deg, rgba(var(--v-theme-on-surface), 0.035) 1px, transparent 1px);
  background-size: auto, 44px 44px, 44px 44px;
}
.setup-shell { position: relative; z-index: 1; width: min(100%, 780px); }
.setup-card {
  overflow: hidden;
  border: 1px solid rgba(var(--v-theme-on-surface), 0.1) !important;
  border-radius: 8px !important;
  background: rgba(var(--v-theme-surface), 0.9) !important;
  box-shadow: 0 24px 70px rgba(0, 0, 0, 0.12), inset 0 1px rgba(255, 255, 255, 0.45) !important;
  backdrop-filter: blur(18px) saturate(135%);
}
.setup-topbar { min-height: 76px; display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 14px 22px; }
.setup-brand-row { display: flex; align-items: center; gap: 12px; min-width: 0; }
.setup-brand-row div { display: grid; min-width: 0; }
.setup-brand-row strong { font-size: 1.1rem; }
.setup-brand-row span { color: rgba(var(--v-theme-on-surface), 0.58); font-size: 0.76rem; }
.setup-logo { flex: 0 0 auto; }
.setup-content { min-height: 430px; padding: 30px 38px 22px !important; }
.setup-step { display: grid; align-content: center; gap: 28px; min-height: 350px; }
.final-step { align-content: start; gap: 20px; }
.step-heading { max-width: 580px; margin-inline: auto; text-align: center; }
.step-heading h1 { margin: 10px 0 8px; font-size: 2rem; line-height: 1.25; letter-spacing: 0; }
.step-heading p { margin: 0; color: rgba(var(--v-theme-on-surface), 0.64); line-height: 1.65; }
.step-heading.compact h1 { font-size: 1.55rem; }
.feature-list { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; }
.feature-row { min-width: 0; display: flex; align-items: flex-start; gap: 12px; padding: 16px; border-top: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); }
.feature-row div { min-width: 0; }
.feature-row strong, .feature-row span { display: block; }
.feature-row strong { font-size: 0.9rem; }
.feature-row span { margin-top: 5px; color: rgba(var(--v-theme-on-surface), 0.6); font-size: 0.78rem; line-height: 1.5; }
.role-toggle { display: grid !important; grid-template-columns: repeat(3, minmax(0, 1fr)); width: 100%; height: auto !important; }
.role-toggle :deep(.v-btn) { min-width: 0 !important; min-height: 52px; padding-inline: 12px; letter-spacing: 0; }
.role-hint, .role-summary { display: flex; align-items: center; gap: 12px; padding: 14px 16px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 7px; background: rgba(var(--v-theme-surface-variant), 0.25); }
.role-hint span { color: rgba(var(--v-theme-on-surface), 0.68); font-size: 0.84rem; line-height: 1.55; }
.role-summary div { display: grid; flex: 1; min-width: 0; }
.role-summary span { color: rgba(var(--v-theme-on-surface), 0.58); font-size: 0.72rem; }
.role-summary strong { font-size: 0.9rem; }
.field-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); column-gap: 12px; }
.field-grid > :first-child { grid-column: 1 / -1; }
.final-actions { display: flex; align-items: center; gap: 10px; margin-top: 4px; }
.setup-footer { min-height: 72px; display: grid; grid-template-columns: 1fr minmax(150px, 210px) 1fr; align-items: center; gap: 14px; padding: 12px 22px 16px; }
.setup-footer > :first-child { justify-self: start; }
.setup-footer > :last-child { justify-self: end; }
.language-select { width: 100%; }
@media (max-width: 640px) {
  .setup-page { place-items: start center; padding: 12px; }
  .setup-topbar { min-height: 66px; padding: 10px 14px; }
  .setup-content { min-height: 0; padding: 22px 16px 14px !important; }
  .step-heading h1 { font-size: 1.55rem; }
  .setup-step { min-height: 400px; gap: 20px; }
  .feature-list, .field-grid { grid-template-columns: minmax(0, 1fr); }
  .feature-row { padding: 13px 6px; }
  .role-toggle { grid-template-columns: minmax(0, 1fr); }
  .role-toggle :deep(.v-btn) { justify-content: flex-start; }
  .field-grid > :first-child { grid-column: auto; }
  .final-actions { flex-wrap: wrap; }
  .final-actions .v-spacer { display: none; }
  .final-actions .v-btn { flex: 1 1 140px; }
  .setup-footer { grid-template-columns: 1fr 1fr; }
  .language-select { grid-column: 1 / -1; grid-row: 1; max-width: 210px; justify-self: center; }
  .setup-footer > :first-child, .setup-footer > :last-child { grid-row: 2; }
}
</style>
