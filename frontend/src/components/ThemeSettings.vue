<template>
  <div class="theme-settings">
    <div class="text-subtitle-2 font-weight-bold mb-3" style="letter-spacing: 0.02em;">{{ $t('setting.themeSection') }}</div>
    <div class="theme-settings__hint">{{ $t('setting.themeHint') }}</div>
    <theme-picker />

    <v-row class="mt-3">
      <template v-if="appearance.theme === systemTheme">
        <v-col cols="12" sm="6" md="4">
          <v-select
            :model-value="appearance.themeLight"
            :items="lightItems"
            :label="$t('setting.themeSystemLight')"
            hide-details
            @update:model-value="(v: string) => setAppearance('themeLight', v)"
          ></v-select>
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <v-select
            :model-value="appearance.themeDark"
            :items="darkItems"
            :label="$t('setting.themeSystemDark')"
            hide-details
            @update:model-value="(v: string) => setAppearance('themeDark', v)"
          ></v-select>
        </v-col>
      </template>

      <v-col cols="12" md="8">
        <div class="ui-choice-field">
          <div class="ui-choice-label">{{ $t('setting.themeAccent') }}</div>
          <div class="accent-row">
            <button
              v-for="color in accentChoices"
              :key="color || 'theme'"
              type="button"
              class="accent-swatch"
              :class="{ 'accent-swatch--active': appearance.accent === color, 'accent-swatch--auto': !color }"
              :style="color ? { background: color } : undefined"
              :title="color || $t('setting.themeAccentAuto')"
              @click="setAppearance('accent', color)"
            >
              <v-icon v-if="!color" icon="mdi-palette-swatch-outline" size="16" />
              <v-icon v-else-if="appearance.accent === color" icon="mdi-check" size="16" color="white" />
            </button>
            <label
              class="accent-swatch accent-swatch--custom"
              :class="{ 'accent-swatch--active': customAccent }"
              :style="customAccent ? { background: appearance.accent } : undefined"
              :title="$t('setting.themeAccentCustom')"
            >
              <v-icon :icon="customAccent ? 'mdi-check' : 'mdi-eyedropper-variant'" size="16" :color="customAccent ? 'white' : undefined" />
              <input
                type="color"
                :value="appearance.accent || '#2563eb'"
                @input="(e) => setAppearance('accent', (e.target as HTMLInputElement).value.toUpperCase())"
              />
            </label>
          </div>
        </div>
      </v-col>

      <v-col cols="12" sm="6" md="4">
        <div class="ui-choice-field">
          <div class="ui-choice-label">{{ $t('setting.uiRadius') }}</div>
          <div class="ui-choice-group">
            <button
              v-for="option in radiusOptions"
              :key="option.value"
              type="button"
              class="ui-choice-button"
              :class="{ 'is-active': appearance.radius === option.value }"
              @click="setAppearance('radius', option.value)"
            >
              {{ option.title }}
            </button>
          </div>
        </div>
      </v-col>
    </v-row>
  </div>
</template>

<script lang="ts" setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import ThemePicker from '@/components/ThemePicker.vue'
import { accentChoices, appearance, setAppearance, systemTheme, themeChoices } from '@/plugins/themes'

const { t } = useI18n()

const lightItems = computed(() => themeChoices.filter(th => !th.dark).map(th => ({ value: th.value, title: t('theme.' + th.value) })))
const darkItems = computed(() => themeChoices.filter(th => th.dark).map(th => ({ value: th.value, title: t('theme.' + th.value) })))
const customAccent = computed(() => appearance.accent !== '' && !accentChoices.includes(appearance.accent))

const radiusOptions = computed(() => [
  { value: 'auto', title: t('setting.uiRadiusAuto') },
  { value: 'sharp', title: t('setting.uiRadiusSharp') },
  { value: 'round', title: t('setting.uiRadiusRound') },
])
</script>

<style scoped>
.theme-settings {
  margin-bottom: 20px;
  padding-bottom: 16px;
  border-bottom: 1px solid rgba(var(--v-theme-on-surface), 0.08);
}

.theme-settings__hint {
  margin: -8px 0 10px;
  font-size: 12px;
  color: rgba(var(--v-theme-on-surface), 0.55);
}

.accent-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.accent-swatch {
  position: relative;
  display: inline-grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border: 2px solid rgb(var(--v-theme-surface));
  border-radius: 50%;
  box-shadow: 0 0 0 1px rgba(var(--v-theme-on-surface), 0.16);
  cursor: pointer;
  transition: transform 0.12s ease, box-shadow 0.12s ease;
}

.accent-swatch:hover {
  transform: scale(1.08);
}

.accent-swatch--active {
  box-shadow: 0 0 0 2px rgb(var(--v-theme-on-surface));
}

.accent-swatch--auto,
.accent-swatch--custom {
  background: rgba(var(--v-theme-on-surface), 0.06);
  color: rgba(var(--v-theme-on-surface), 0.72);
}

.accent-swatch--custom input {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0;
  cursor: pointer;
}
</style>
