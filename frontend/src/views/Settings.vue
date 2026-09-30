<template>
  <v-card :loading="loading">
    <v-tabs
    v-model="tab"
    color="primary"
    align-tabs="center"
    show-arrows
  >
    <v-tab value="t0">{{ $t('setting.serverPanel') }}</v-tab>
    <v-tab value="t1">{{ $t('setting.interface') }}</v-tab>
    <v-tab value="t2">{{ $t('setting.sub') }}</v-tab>
    <v-tab value="t3">{{ $t('setting.jsonSub') }}</v-tab>
    <v-tab value="t4">{{ $t('setting.clashSub') }}</v-tab>
    <v-tab value="t5">{{ $t('setting.network') }}</v-tab>
  </v-tabs>
  <v-card-text>
    <v-row class="settings-actions" align="center" justify="center" style="margin-bottom: 10px;">
      <v-col cols="auto">
        <v-btn color="primary" @click="save" :loading="loading" :disabled="!stateChange">
          {{ $t('actions.save') }}
        </v-btn>
      </v-col>
      <v-col cols="auto">
        <v-btn variant="outlined" color="warning" @click="restartApp" :loading="loading" :disabled="stateChange">
          {{ $t('actions.restartApp') }}
        </v-btn>
      </v-col>
    </v-row>
    <v-window v-model="tab">
      <v-window-item value="t0">
        <section class="role-section">
          <div class="role-header">
            <div>
              <div class="text-subtitle-1 font-weight-bold">{{ $t('setting.runtimeRoleTitle') }}</div>
              <div class="text-body-2 text-medium-emphasis">{{ $t('setting.runtimeRoleHint') }}</div>
            </div>
            <v-chip
              :color="controllerMode.profile === 'full' ? 'primary' : controllerMode.profile === 'monitor' ? 'info' : 'default'"
              :prepend-icon="controllerMode.profile === 'monitor' ? 'mdi-monitor-eye' : controllerMode.profile === 'full' ? 'mdi-server-network' : 'mdi-server-outline'"
            >
              {{ controllerProfileLabel }}
            </v-chip>
          </div>
          <v-alert
            v-if="controllerMode.lite"
            type="info"
            variant="tonal"
            density="compact"
            class="mt-3"
            icon="mdi-feather"
          >
            {{ $t('setting.roleLiteActive') }}
          </v-alert>
          <v-alert
            v-else-if="controllerMode.profile === 'client' && controllerMode.can_enable && hostBelowCluster"
            type="info"
            variant="tonal"
            density="compact"
            class="mt-3"
            icon="mdi-feather"
          >
            {{ $t('setting.roleLiteNotice') }}
          </v-alert>
          <v-alert
            :type="controllerMode.profile === 'client' ? 'success' : 'info'"
            variant="tonal"
            density="compact"
            class="mt-3"
          >
            {{ controllerProfileHint }}
          </v-alert>
          <div class="role-capacity">
            <span>{{ controllerMode.cpu_cores }} CPU</span>
            <span>{{ formatMemory(controllerMode.memory_bytes) }}</span>
            <span>{{ $t('agent.totalServers') }}: {{ controllerMode.agent_count }}</span>
          </div>
          <v-alert
            v-if="controllerMode.profile === 'client' && !controllerMode.can_enable"
            type="warning"
            variant="tonal"
            density="compact"
            class="mt-3"
          >
            {{ $t('setting.roleResourceBlocked') }}
          </v-alert>
          <div class="role-actions">
            <v-btn-toggle :model-value="controllerMode.profile" color="primary" mandatory divided class="role-profile-toggle">
              <v-btn
                v-for="option in controllerProfileOptions"
                :key="option.value"
                :value="option.value"
                :prepend-icon="option.icon"
                :loading="controllerModeLoading && pendingControllerProfile === option.value"
                :disabled="controllerMode.profile === 'client' && option.value !== 'client' && !controllerMode.can_enable"
                @click="setControllerMode(option.value)"
              >{{ option.title }}</v-btn>
            </v-btn-toggle>
            <span class="text-caption text-medium-emphasis">
              {{ controllerMode.can_enable ? $t(hostBelowCluster ? 'setting.roleResourceLite' : 'setting.roleResourceReady') : liteFloorLabel }}
            </span>
          </div>
        </section>

        <v-divider class="my-5" opacity="40"></v-divider>
        <section class="xray-install-section">
          <div class="xray-install-header">
            <div>
              <div class="text-subtitle-1 font-weight-bold">{{ $t('setting.xrayInstallTitle') }}</div>
              <div class="text-body-2 text-medium-emphasis">{{ $t('setting.xrayInstallHint') }}</div>
            </div>
            <div class="xray-install-status">
              <v-chip
                :color="xrayInstall.installed ? 'success' : 'default'"
                :prepend-icon="xrayInstall.installed ? 'mdi-check-circle' : 'mdi-download-circle-outline'"
              >
                {{ xrayInstall.installed ? $t('setting.xrayInstalled') : $t('setting.xrayNotInstalled') }}
              </v-chip>
              <v-chip
                v-if="xrayInstall.installed"
                :color="xrayInstall.disabled ? 'warning' : 'primary'"
                :prepend-icon="xrayInstall.disabled ? 'mdi-power-plug-off-outline' : 'mdi-power-plug-outline'"
              >
                {{ xrayInstall.disabled ? $t('setting.xrayDisabled') : $t('setting.xrayEnabled') }}
              </v-chip>
              <v-chip
                v-if="xrayInstall.installed && !xrayInstall.disabled"
                :color="xrayInstall.running ? 'success' : 'default'"
                :prepend-icon="xrayInstall.running ? 'mdi-play-circle' : 'mdi-stop-circle-outline'"
              >
                {{ xrayInstall.running ? $t('setting.xrayRunning') : $t('setting.xrayStopped') }}
              </v-chip>
            </div>
          </div>
          <v-alert
            v-if="!xrayInstall.supported"
            type="warning"
            variant="tonal"
            density="compact"
            class="mt-3"
          >
            {{ $t('setting.xrayInstallUnsupported') }}
          </v-alert>
          <template v-else>
            <v-alert
              v-if="controllerMode.lite"
              type="info"
              variant="tonal"
              density="compact"
              class="mt-3"
            >
              {{ $t('setting.xrayLiteControllerHint') }}
            </v-alert>
            <v-alert
              v-else-if="xrayInstall.low_resource"
              type="warning"
              variant="tonal"
              density="compact"
              class="mt-3"
            >
              {{ $t('setting.xrayLowResourceHint') }}
            </v-alert>
            <div class="xray-install-facts">
              <v-chip size="small" color="primary" variant="tonal" prepend-icon="mdi-shield-check-outline">
                {{ $t('setting.xraySingboxDefault') }}
              </v-chip>
              <v-chip size="small" :color="xrayInstall.on_demand ? 'success' : 'default'" variant="tonal" prepend-icon="mdi-power-settings">
                {{ $t('setting.xrayOnDemand') }}
              </v-chip>
              <v-chip v-if="xrayInstall.exclusive_run" size="small" color="warning" variant="tonal" prepend-icon="mdi-swap-horizontal-bold">
                {{ $t('setting.xrayExclusiveRuntime') }}
              </v-chip>
              <span>{{ xrayInstall.cpu_cores }} CPU</span>
              <span>{{ formatMemory(xrayInstall.memory_bytes) }}</span>
            </div>
            <v-row v-if="xrayInstall.installed" class="mt-1">
              <v-col cols="12" sm="5">
                <v-text-field :model-value="xrayInstall.version || '-'" :label="$t('setting.xrayVersion')" readonly hide-details />
              </v-col>
              <v-col cols="12" sm="7">
                <v-text-field :model-value="xrayInstall.path || '-'" :label="$t('setting.xrayPath')" readonly hide-details />
              </v-col>
            </v-row>
            <v-alert
              v-if="xrayInstall.installed && !xrayInstall.has_inbounds"
              type="info"
              variant="tonal"
              density="compact"
              class="mt-3"
            >
              {{ $t('setting.xrayNoInboundsHint') }}
            </v-alert>
            <v-alert
              v-else-if="xrayInstall.installed && xrayInstall.has_inbounds"
              type="info"
              variant="tonal"
              density="compact"
              class="mt-3"
            >
              {{ $t('setting.xrayUninstallProtected') }}
            </v-alert>
            <v-alert
              v-if="xrayInstall.install.state !== 'idle'"
              :type="xrayInstall.install.state === 'failed' ? 'error' : xrayInstall.install.state === 'success' ? 'success' : 'info'"
              variant="tonal"
              density="compact"
              class="mt-3"
            >
              {{ xrayInstallStateText }}
            </v-alert>
            <v-alert v-if="xrayGitHubFailed" type="info" variant="tonal" density="compact" class="mt-3" icon="mdi-cloud-alert-outline">
              {{ $t('setting.githubUnreachableHint') }}
            </v-alert>
            <div class="xray-install-actions">
              <v-btn
                color="primary"
                prepend-icon="mdi-download"
                :loading="xrayInstallLoading || xrayInstallRunning"
                :disabled="!xrayInstall.can_install || xrayInstallActionLoading || xrayInstall.running || controllerMode.lite"
                @click="requestXrayInstall"
              >
                {{ xrayInstall.installed ? $t('setting.xrayReinstall') : $t('setting.xrayInstallNow') }}
              </v-btn>
              <template v-if="xrayInstall.installed">
                <v-btn
                  :color="xrayInstall.disabled ? 'primary' : 'warning'"
                  :variant="xrayInstall.disabled ? 'elevated' : 'tonal'"
                  :prepend-icon="xrayInstall.disabled ? 'mdi-power-plug-outline' : 'mdi-power-plug-off-outline'"
                  :loading="xrayInstallAction === 'enabled'"
                  :disabled="xrayInstallActionLoading || xrayInstallRunning || (xrayInstall.disabled && controllerMode.lite)"
                  @click="setXrayEnabled(xrayInstall.disabled)"
                >
                  {{ xrayInstall.disabled ? $t('setting.xrayEnable') : $t('setting.xrayDisable') }}
                </v-btn>
                <v-btn
                  v-if="!xrayInstall.disabled"
                  :color="xrayInstall.running ? 'warning' : 'success'"
                  variant="tonal"
                  :prepend-icon="xrayInstall.running ? 'mdi-stop' : 'mdi-play'"
                  :loading="xrayInstallAction === 'runtime'"
                  :disabled="xrayInstallActionLoading || xrayInstallRunning || (!xrayInstall.running && !xrayInstall.has_inbounds)"
                  @click="xrayInstall.running ? stopXray() : startXray()"
                >
                  {{ xrayInstall.running ? $t('setting.xrayStop') : $t('setting.xrayStart') }}
                </v-btn>
                <v-btn
                  v-if="!xrayInstall.has_inbounds"
                  variant="outlined"
                  prepend-icon="mdi-plus-circle-outline"
                  :disabled="xrayInstallActionLoading || xrayInstallRunning"
                  @click="openXrayInbounds"
                >
                  {{ $t('setting.xrayAddInbound') }}
                </v-btn>
                <v-btn
                  color="error"
                  variant="text"
                  prepend-icon="mdi-delete-outline"
                  :loading="xrayInstallAction === 'uninstall'"
                  :disabled="xrayInstallActionLoading || xrayInstallRunning || xrayInstall.has_inbounds"
                  @click="uninstallXray"
                >
                  {{ $t('setting.xrayUninstall') }}
                </v-btn>
              </template>
            </div>
            <div class="text-caption text-medium-emphasis mt-2">{{ $t('setting.xrayInstallSafetyHint') }}</div>
          </template>
        </section>

        <v-divider class="my-5" opacity="40"></v-divider>
        <section class="version-section">
          <div class="version-header">
            <div>
              <div class="text-subtitle-1 font-weight-bold">{{ $t('setting.versionCheckTitle') }}</div>
              <div class="text-body-2 text-medium-emphasis">{{ $t('setting.versionCheckHint') }}</div>
            </div>
            <v-btn
              prepend-icon="mdi-refresh"
              variant="tonal"
              :loading="versionLoading"
              @click="loadVersion"
            >
              {{ $t('setting.versionCheckNow') }}
            </v-btn>
          </div>
          <v-row class="mt-3">
            <v-col cols="12" sm="6" md="3">
              <v-text-field
                :model-value="versionInfo.current || '-'"
                :label="$t('setting.versionCurrent')"
                readonly
                hide-details
              ></v-text-field>
            </v-col>
            <v-col cols="12" sm="6" md="3">
              <v-text-field
                :model-value="versionInfo.latest || '-'"
                :label="$t('setting.versionLatest')"
                readonly
                hide-details
              ></v-text-field>
            </v-col>
            <v-col cols="12" sm="6" md="6" class="d-flex align-center">
              <v-alert
                v-if="versionInfo.update_available"
                type="info"
                variant="tonal"
                density="compact"
                class="version-alert"
              >
                {{ $t('setting.versionAvailable') }}
              </v-alert>
              <v-alert
                v-else-if="versionInfo.latest"
                type="success"
                variant="tonal"
                density="compact"
                class="version-alert"
              >
                {{ $t('setting.versionUpToDate') }}
              </v-alert>
            </v-col>
          </v-row>
          <v-alert
            v-if="versionInfo.update.state !== 'idle'"
            :type="versionInfo.update.state === 'failed' ? 'error' : 'info'"
            variant="tonal"
            density="compact"
            class="mt-3"
          >
            {{ versionStateText }}
          </v-alert>
          <v-alert
            v-if="versionInfo.update_available && !versionInfo.can_update"
            type="warning"
            variant="tonal"
            density="compact"
            class="mt-3"
          >
            {{ versionInfo.capability || $t('setting.versionUpdateUnavailable') }}
          </v-alert>
          <div class="version-actions">
            <v-btn
              color="primary"
              prepend-icon="mdi-download"
              :loading="versionUpdating"
              :disabled="!versionInfo.update_available || !versionInfo.can_update || versionLoading"
              @click="requestUpdate"
            >
              {{ $t('setting.versionUpdateNow') }}
            </v-btn>
            <span class="text-caption text-medium-emphasis">{{ $t('setting.versionUpdateHint') }}</span>
          </div>
          <v-alert v-if="versionGitHubFailed" type="info" variant="tonal" density="compact" class="mt-3" icon="mdi-cloud-alert-outline">
            {{ $t('setting.githubUnreachableHint') }}
          </v-alert>
          <v-row class="mt-3">
            <v-col cols="12" lg="10">
              <v-select
                v-model="settings.downloadLine"
                :items="downloadLineOptions"
                :label="$t('setting.downloadLine')"
                :hint="$t('setting.downloadLineHint')"
                persistent-hint
                prepend-inner-icon="mdi-map-marker-path"
              ></v-select>
            </v-col>
            <v-col cols="12" lg="10">
              <v-text-field
                v-model="settings.githubMirror"
                :label="$t('setting.githubMirror')"
                placeholder="https://ghfast.top/"
                :hint="$t('setting.githubMirrorHint')"
                persistent-hint
                clearable
                prepend-inner-icon="mdi-cloud-sync-outline"
              ></v-text-field>
            </v-col>
          </v-row>
        </section>

        <v-divider class="my-5" opacity="40"></v-divider>
        <section class="reverse-proxy-section">
          <div class="reverse-proxy-header">
            <div>
              <div class="text-subtitle-1 font-weight-bold">{{ $t('setting.reverseProxyTitle') }}</div>
              <div class="text-body-2 text-medium-emphasis">{{ $t('setting.reverseProxyHint') }}</div>
            </div>
            <div class="reverse-proxy-status">
              <v-chip
                size="small"
                :color="reverseProxyStatus.running ? 'success' : 'warning'"
                :prepend-icon="reverseProxyStatus.running ? 'mdi-check-circle' : 'mdi-alert-circle-outline'"
              >
                {{ reverseProxyStatus.running ? $t('setting.reverseProxyRunning') : $t('setting.reverseProxyStopped') }}
              </v-chip>
              <v-chip
                size="small"
                :color="reverseProxyStatus.managed ? 'primary' : 'default'"
                prepend-icon="mdi-shield-check-outline"
              >
                {{ reverseProxyStatus.managed ? $t('setting.reverseProxyManaged') : $t('setting.reverseProxyUnmanaged') }}
              </v-chip>
              <v-btn
                icon="mdi-refresh"
                size="small"
                variant="text"
                :loading="reverseProxyLoading"
                :aria-label="$t('setting.reverseProxyRefresh')"
                @click="loadReverseProxy"
              ></v-btn>
            </div>
          </div>

          <v-alert
            v-if="!reverseProxyStatus.supported"
            type="warning"
            variant="tonal"
            density="compact"
            class="mt-4"
          >
            {{ $t('setting.reverseProxyUnsupported') }}
          </v-alert>
          <v-alert
            v-else-if="reverseProxyStatus.running && !reverseProxyStatus.managed"
            type="warning"
            variant="tonal"
            density="compact"
            class="mt-4"
          >
            {{ $t('setting.reverseProxyCustomConfig') }}
          </v-alert>
          <v-alert
            v-else-if="!selectedReverseProxyInstalled"
            type="info"
            variant="tonal"
            density="compact"
            class="mt-4"
          >
            {{ $t('setting.reverseProxyNotInstalled') }}
          </v-alert>

          <v-row class="mt-1">
            <v-col cols="12" sm="6" md="4">
              <v-select
                v-model="reverseProxyConfig.engine"
                :items="reverseProxyEngineOptions"
                item-title="title"
                item-value="value"
                :label="$t('setting.reverseProxyEngine')"
                hide-details
              ></v-select>
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field
                v-model.trim="reverseProxyConfig.domain"
                :label="$t('setting.reverseProxyDomain')"
                :placeholder="$t('setting.reverseProxyDomainPlaceholder')"
                hide-details
                clearable
              ></v-text-field>
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field
                :model-value="reverseProxyUpstream"
                :label="$t('setting.reverseProxyUpstream')"
                prepend-inner-icon="mdi-server-network"
                readonly
                hide-details
              ></v-text-field>
            </v-col>
            <v-col cols="12">
              <v-text-field
                :model-value="reverseProxyPublicURL"
                :label="$t('setting.reverseProxyPublicURL')"
                prepend-inner-icon="mdi-open-in-new"
                readonly
                hide-details
              ></v-text-field>
            </v-col>
          </v-row>

          <div class="reverse-proxy-actions">
            <v-btn
              color="primary"
              prepend-icon="mdi-shield-sync-outline"
              :loading="reverseProxyLoading"
              :disabled="!canApplyReverseProxy"
              @click="applyReverseProxy"
            >
              {{ $t('setting.reverseProxyApply') }}
            </v-btn>
            <span class="text-caption text-medium-emphasis">{{ $t('setting.reverseProxyRestartHint') }}</span>
          </div>
        </section>

        <v-divider class="my-5" opacity="40"></v-divider>
        <div class="text-subtitle-2 font-weight-bold mb-3">{{ $t('setting.panelBuiltinServer') }}</div>
        <v-alert type="info" variant="tonal" density="compact" class="mb-3">
          {{ $t('setting.frontendAccessHint') }}
        </v-alert>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field :model-value="frontendAddress" :label="$t('setting.frontendAddress')" readonly hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field :model-value="frontendPort" :label="$t('setting.frontendPort')" readonly hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field :model-value="frontendPath" :label="$t('setting.frontendPath')" readonly hide-details></v-text-field>
          </v-col>
          <v-col cols="12">
            <v-text-field :model-value="backendApiURL" :label="$t('setting.backendApiUrl')" readonly hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model.number="sessionMaxAge"
              min="0"
              :label="$t('setting.sessionAge')"
              :suffix="$t('date.m')"
              hide-details
              ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model.number="trafficAge"
              min="0"
              :label="$t('setting.trafficAge')"
              :suffix="$t('date.d')"
              hide-details
              ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model.number="statsBucketSeconds"
              min="1"
              :label="$t('setting.statsBucketSeconds')"
              :suffix="$t('date.s')"
              v-tooltip:top="$t('setting.statsBucketSecondsHint')"
              hide-details
              ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.timeLocation" :label="$t('setting.timeLoc')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              v-model="settings.globalReset"
              :label="$t('setting.globalReset')"
              v-tooltip:top="$t('setting.globalResetHint')"
              hide-details
              placeholder="0 0 1 * *"></v-text-field>
          </v-col>
        </v-row>
      </v-window-item>

      <v-window-item value="t1">
        <theme-settings />
        <div class="text-subtitle-2 font-weight-bold mb-3" style="letter-spacing: 0.02em;">{{ $t('setting.uiCustomization') }}</div>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <div class="ui-choice-field">
              <div class="ui-choice-label">{{ $t('setting.menuPosition') }}</div>
              <div class="ui-choice-group">
                <button
                  v-for="option in menuPositionOptions"
                  :key="option.value"
                  type="button"
                  class="ui-choice-button"
                  :class="{ 'is-active': menuPositionModel === option.value }"
                  @click="menuPositionModel = option.value"
                >
                  {{ option.title }}
                </button>
              </div>
            </div>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <div class="ui-choice-field">
              <div class="ui-choice-label">{{ $t('setting.uiStyle') }}</div>
              <div class="ui-choice-group">
                <button
                  v-for="option in uiStyleOptions"
                  :key="option.value"
                  type="button"
                  class="ui-choice-button"
                  :class="{ 'is-active': uiStyleModel === option.value }"
                  @click="uiStyleModel = option.value"
                >
                  {{ option.title }}
                </button>
              </div>
            </div>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <div class="ui-choice-field">
              <div class="ui-choice-label">{{ $t('setting.uiDensity') }}</div>
              <div class="ui-choice-group">
                <button
                  v-for="option in uiDensityOptions"
                  :key="option.value"
                  type="button"
                  class="ui-choice-button"
                  :class="{ 'is-active': uiDensityModel === option.value }"
                  @click="uiDensityModel = option.value"
                >
                  {{ option.title }}
                </button>
              </div>
            </div>
          </v-col>
          <!-- The panel style has no background image. -->
          <template v-if="uiStyleModel !== 'panel'">
          <v-col cols="12" sm="6" md="4">
            <div class="ui-choice-field">
              <div class="ui-choice-label">{{ $t('setting.bgPreset') }}</div>
              <div class="ui-choice-group">
                <button
                  v-for="option in bgPresetOptions"
                  :key="option.value"
                  type="button"
                  class="ui-choice-button"
                  :class="{ 'is-active': bgPresetModel === option.value }"
                  @click="bgPresetModel = option.value"
                >
                  {{ option.title }}
                </button>
              </div>
            </div>
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="bgPresetModel === 'custom'">
            <v-text-field
              v-model="bgImageModel"
              :label="$t('setting.bgImage')"
              :placeholder="$t('setting.bgImagePlaceholder')"
              hide-details
              clearable
              @click:clear="bgImageModel = ''"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="bgPresetModel === 'custom'">
            <v-file-input
              accept="image/png,image/jpeg,image/webp,image/gif"
              prepend-icon="mdi-image-plus"
              :label="$t('setting.bgUpload')"
              hide-details
              clearable
              @update:model-value="handleBgFile"
            ></v-file-input>
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="previewBackground">
            <v-img :src="previewBackground" height="76" cover class="rounded-lg app-bg-preview" />
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="bgPresetModel !== 'none'">
            <div class="ui-choice-field">
              <div class="ui-choice-label">{{ $t('setting.bgFit') }}</div>
              <div class="ui-choice-group">
                <button
                  v-for="option in bgFitOptions"
                  :key="option.value"
                  type="button"
                  class="ui-choice-button"
                  :class="{ 'is-active': bgFitModel === option.value }"
                  @click="bgFitModel = option.value"
                >
                  {{ option.title }}
                </button>
              </div>
            </div>
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="bgPresetModel !== 'none'">
            <div class="ui-choice-field">
              <div class="ui-choice-label">{{ $t('setting.bgPosition') }}</div>
              <div class="ui-choice-group">
                <button
                  v-for="option in bgPositionOptions"
                  :key="option.value"
                  type="button"
                  class="ui-choice-button"
                  :class="{ 'is-active': bgPositionModel === option.value }"
                  @click="bgPositionModel = option.value"
                >
                  {{ option.title }}
                </button>
              </div>
            </div>
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="bgPresetModel !== 'none'">
            <div class="ui-range-field">
              <div class="ui-range-header">
                <span>{{ $t('setting.bgBlur') || 'Background Blur' }}</span>
                <span class="ui-range-value">{{ bgBlurModel }}px</span>
              </div>
              <input
                v-model.number="bgBlurModel"
                class="ui-range"
                type="range"
                min="0"
                max="20"
                step="1"
                :style="rangeStyle(bgBlurModel, 0, 20)"
              />
            </div>
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="bgPresetModel !== 'none'">
            <div class="ui-range-field">
              <div class="ui-range-header">
                <span>{{ $t('setting.bgOpacity') || 'Background Opacity' }}</span>
                <span class="ui-range-value">{{ bgOpacityModel }}%</span>
              </div>
              <input
                v-model.number="bgOpacityModel"
                class="ui-range"
                type="range"
                min="5"
                max="100"
                step="1"
                :style="rangeStyle(bgOpacityModel, 5, 100)"
              />
            </div>
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="bgPresetModel !== 'none'">
            <div class="ui-range-field">
              <div class="ui-range-header">
                <span>{{ $t('setting.bgSaturate') }}</span>
                <span class="ui-range-value">{{ bgSaturateModel }}%</span>
              </div>
              <input
                v-model.number="bgSaturateModel"
                class="ui-range"
                type="range"
                min="50"
                max="180"
                step="5"
                :style="rangeStyle(bgSaturateModel, 50, 180)"
              />
            </div>
          </v-col>
          </template>
          <v-col cols="12" sm="6" md="4">
            <v-btn color="primary" variant="tonal" prepend-icon="mdi-restore" @click="resetUiPrefs">
              {{ $t('setting.resetUi') }}
            </v-btn>
          </v-col>
        </v-row>
      </v-window-item>

      <v-window-item value="t2">
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-switch color="primary" v-model="subEncode" :label="$t('setting.subEncode')" hide-details />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-switch color="primary" v-model="subShowInfo" :label="$t('setting.subInfo')" hide-details />
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subListen" :label="$t('setting.addr')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model.number="subPort"
              min="1"
              :label="$t('setting.port')"
              hide-details></v-text-field>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subKeyFile" :label="$t('setting.sslKey')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subCertFile" :label="$t('setting.sslCert')" hide-details></v-text-field>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subDomain" :label="$t('setting.domain')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subPath" :label="$t('setting.path')" hide-details></v-text-field>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model.number="subUpdates"
              min="0"
              :label="$t('setting.update')"
              hide-details
              ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subURI" :label="$t('setting.subUri')" hide-details></v-text-field>
          </v-col>
        </v-row>
      </v-window-item>

      <v-window-item value="t3">
        <SubJsonExtVue :settings="settings" />
      </v-window-item>

      <v-window-item value="t4">
        <SubClashExtVue :settings="settings" />
      </v-window-item>

      <v-window-item value="t5">
        <v-card variant="tonal" class="mb-4">
          <v-card-title>{{ $t('setting.congestion') }}</v-card-title>
          <v-card-text>
            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-select
                  v-model="bbrVersion"
                  :label="$t('setting.bbrVersion')"
                  :items="bbrOptions"
                  item-title="title"
                  item-value="value"
                  hide-details
                ></v-select>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-select
                  v-model="qdisc"
                  :label="$t('setting.qdisc')"
                  :items="qdiscOptions"
                  item-title="title"
                  item-value="value"
                  hide-details
                ></v-select>
              </v-col>
            </v-row>
            <v-row>
              <v-col cols="12">
                <v-btn
                  color="primary"
                  variant="tonal"
                  :loading="sysctlLoading"
                  @click="applyCongestion"
                >
                  <v-icon start icon="mdi-cog"></v-icon>
                  {{ $t('setting.applySysctl') }}
                </v-btn>
                <v-chip v-if="sysctlResult" :color="sysctlError ? 'error' : 'success'" class="ml-2" size="small">
                  {{ sysctlResult }}
                </v-chip>
              </v-col>
            </v-row>
            <v-row v-if="sysctlMessages.length > 0">
              <v-col cols="12">
                <v-card variant="outlined" density="compact">
                  <v-card-text>
                    <div v-for="msg in sysctlMessages" :key="msg" class="text-caption">{{ msg }}</div>
                  </v-card-text>
                </v-card>
              </v-col>
            </v-row>
          </v-card-text>
        </v-card>
      </v-window-item>
    </v-window>
  </v-card-text>
</v-card>
</template>

<script lang="ts" setup>
import { i18n } from '@/locales'
import { Ref, computed, inject, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import ThemeSettings from '@/components/ThemeSettings.vue'
import HttpUtils from '@/plugins/httputil'
import { FindDiff } from '@/plugins/utils'
import SubJsonExtVue from '@/components/SubJsonExt.vue'
import SubClashExtVue from '@/components/SubClashExt.vue'
import { push } from 'notivue'
import bgAsset from '@/assets/bg.jpg'
import { backendBaseUrl, resolveFrontendUrl, runtimeConfig } from '@/utils/backend'
import Data from '@/store/modules/data'

type ReverseProxyStatus = {
  supported: boolean
  privileged: boolean
  enabled: boolean
  installed: boolean
  running: boolean
  managed: boolean
  caddyInstalled: boolean
  nginxInstalled: boolean
  engine: string
  domain: string
  panelListen: string
  panelPort: number
  apiListen: string
  panelPath: string
  publicUrl: string
  message: string
}

type VersionInfo = {
  current: string
  latest: string
  update_available: boolean
  asset_available: boolean
  asset: string
  release_url: string
  published_at: string
  can_update: boolean
  capability: string
  update: {
    state: string
    version: string
    message: string
    started: number
  }
}

type XrayInstallStatus = {
  supported: boolean
  can_install: boolean
  installed: boolean
  disabled: boolean
  on_demand: boolean
  running: boolean
  has_inbounds: boolean
  low_resource: boolean
  exclusive_run: boolean
  architecture: string
  version: string
  path: string
  capability: string
  cpu_cores: number
  memory_bytes: number
  install: {
    state: string
    version: string
    message: string
    started: number
  }
}

const emptyVersionInfo = (): VersionInfo => ({
  current: '',
  latest: '',
  update_available: false,
  asset_available: false,
  asset: '',
  release_url: '',
  published_at: '',
  can_update: false,
  capability: '',
  update: { state: 'idle', version: '', message: '', started: 0 },
})

const emptyXrayInstall = (): XrayInstallStatus => ({
  supported: false,
  can_install: false,
  installed: false,
  disabled: true,
  on_demand: false,
  running: false,
  has_inbounds: false,
  low_resource: false,
  exclusive_run: false,
  architecture: '',
  version: '',
  path: '',
  capability: '',
  cpu_cores: 0,
  memory_bytes: 0,
  install: { state: 'idle', version: '', message: '', started: 0 },
})

const emptyReverseProxyStatus = (): ReverseProxyStatus => ({
  supported: false,
  privileged: false,
  enabled: false,
  installed: false,
  running: false,
  managed: false,
  caddyInstalled: false,
  nginxInstalled: false,
  engine: 'caddy',
  domain: '',
  panelListen: '',
  panelPort: 2095,
  apiListen: '',
  panelPath: '/',
  publicUrl: '',
  message: '',
})

const route = useRoute()
const tabs = ['t0', 't1', 't2', 't3', 't4', 't5']
const queryTab = () => (tabs.includes(String(route.query.tab)) ? String(route.query.tab) : '')
// ?tab=t1 opens a tab directly, e.g. from the theme menu in the header.
const tab = ref(queryTab() || "t0")
watch(() => route.query.tab, () => { if (queryTab()) tab.value = queryTab() })

const versionInfo = ref<VersionInfo>(emptyVersionInfo())
const versionLoading = ref(false)
const versionUpdating = ref(false)
const versionTarget = ref('')
let versionPollTimer: number | undefined
const controllerModeLoading = ref(false)
const pendingControllerProfile = ref('')
const xrayInstall = ref<XrayInstallStatus>(emptyXrayInstall())
const xrayInstallLoading = ref(false)
const xrayInstallAction = ref('')
let xrayInstallPollTimer: number | undefined

const router = useRouter()
const dataStore = Data()
const controllerMode = computed(() => dataStore.controllerMode)
const controllerProfileOptions = computed(() => [
  { value: 'client', title: i18n.global.t('setting.roleClient'), icon: 'mdi-server-outline' },
  { value: 'full', title: i18n.global.t('setting.roleFull'), icon: 'mdi-server-network' },
  { value: 'monitor', title: i18n.global.t('setting.roleMonitor'), icon: 'mdi-monitor-eye' },
])
const controllerProfileLabel = computed(() => {
  const label = i18n.global.t(`setting.role${controllerMode.value.profile === 'full' ? 'Full' : controllerMode.value.profile === 'monitor' ? 'Monitor' : 'Client'}`)
  return controllerMode.value.lite ? `${label} · ${i18n.global.t('setting.roleLite')}` : label
})
// Below the cluster minimum the controller runs in lite mode (sing-box only).
const hostBelowCluster = computed(() =>
  controllerMode.value.cpu_cores < controllerMode.value.min_cpu_cores ||
  controllerMode.value.memory_bytes < controllerMode.value.min_memory_bytes)
const liteFloorLabel = computed(() =>
  `${controllerMode.value.lite_min_cpu_cores} CPU / ${Math.round(controllerMode.value.lite_min_memory_bytes / 1024 / 1024)} MiB`)
const controllerProfileHint = computed(() => i18n.global.t(`setting.role${controllerMode.value.profile === 'full' ? 'Full' : controllerMode.value.profile === 'monitor' ? 'Monitor' : 'Client'}Hint`))

const xrayInstallRunning = computed(() => ['downloading', 'installing'].includes(xrayInstall.value.install.state))
const xrayInstallActionLoading = computed(() => xrayInstallAction.value !== '')
const xrayInstallStateText = computed(() => {
  const state = xrayInstall.value.install.state
  if (state === 'downloading') return i18n.global.t('setting.xrayDownloading')
  if (state === 'installing') return i18n.global.t('setting.xrayInstalling')
  if (state === 'success') {
    return i18n.global.t(xrayInstall.value.disabled ? 'setting.xrayInstallSuccessDisabled' : 'setting.xrayInstallSuccess')
  }
  if (state === 'failed') return xrayInstall.value.install.message || i18n.global.t('setting.xrayInstallFailed')
  return xrayInstall.value.install.message
})

const versionStateText = computed(() => {
  const state = versionInfo.value.update.state
  if (state === 'downloading') return i18n.global.t('setting.versionUpdateDownloading')
  if (state === 'installing') return i18n.global.t('setting.versionUpdateInstalling')
  if (state === 'restarting') return i18n.global.t('setting.versionUpdateRestarting')
  if (state === 'success') return i18n.global.t('setting.versionUpdateStarted')
  if (state === 'failed') return versionInfo.value.update.message || i18n.global.t('setting.versionUpdateFailed')
  return versionInfo.value.update.message
})

const uiPreferenceEvent = 'ui-preferences-changed'
const notifyUiPrefs = () => window.dispatchEvent(new Event(uiPreferenceEvent))

type UiPrefs = {
  menuPosition: string
  uiStyle: string
  uiDensity: string
  bgPreset: string
  bgImage: string
  bgBlur: string
  bgOpacity: string
  bgSaturate: string
  bgFit: string
  bgPosition: string
}

const uiPrefChoices = {
  menuPosition: ['side', 'top'],
  uiStyle: ['panel', 'glass', 'solid', 'clear'],
  uiDensity: ['comfortable', 'compact'],
  bgPreset: ['default', 'none', 'custom'],
  bgFit: ['cover', 'contain', 'auto'],
  bgPosition: ['center', 'center top', 'center bottom'],
} as const

const uiPrefValue = (value: unknown) => {
  if (value && typeof value === 'object' && 'value' in value) {
    return String((value as { value?: unknown }).value ?? '')
  }
  return String(value ?? '')
}

const normalizeUiChoice = (value: unknown, fallback: string, choices: readonly string[]) => {
  const next = uiPrefValue(value)
  return choices.includes(next) ? next : fallback
}

const readUiPrefs = (): UiPrefs => ({
  menuPosition: normalizeUiChoice(localStorage.getItem('menuPosition'), 'side', uiPrefChoices.menuPosition),
  uiStyle: normalizeUiChoice(localStorage.getItem('uiStyle'), 'panel', uiPrefChoices.uiStyle),
  uiDensity: normalizeUiChoice(localStorage.getItem('uiDensity'), 'comfortable', uiPrefChoices.uiDensity),
  bgPreset: normalizeUiChoice(localStorage.getItem('bgPreset'), localStorage.getItem('bgImage') ? 'custom' : 'default', uiPrefChoices.bgPreset),
  bgImage: localStorage.getItem('bgImage') || '',
  bgBlur: localStorage.getItem('bgBlur') || '6',
  bgOpacity: localStorage.getItem('bgOpacity') || '40',
  bgSaturate: localStorage.getItem('bgSaturate') || '1.3',
  bgFit: normalizeUiChoice(localStorage.getItem('bgFit'), 'cover', uiPrefChoices.bgFit),
  bgPosition: normalizeUiChoice(localStorage.getItem('bgPosition'), 'center', uiPrefChoices.bgPosition),
})

const uiPrefs = ref<UiPrefs>(readUiPrefs())

const setUiPref = (key: keyof UiPrefs, value: unknown) => {
  const next = uiPrefValue(value)
  uiPrefs.value = { ...uiPrefs.value, [key]: next }
  if (next) localStorage.setItem(key, next)
  else localStorage.removeItem(key)
  notifyUiPrefs()
}

const menuPositionModel = computed({
  get: () => uiPrefs.value.menuPosition,
  set: (v: unknown) => setUiPref('menuPosition', normalizeUiChoice(v, 'side', uiPrefChoices.menuPosition))
})
const menuPositionOptions = [
  { title: i18n.global.t('setting.menuSide'), value: 'side' },
  { title: i18n.global.t('setting.menuTop'), value: 'top' },
]
const uiStyleModel = computed({
  get: () => uiPrefs.value.uiStyle,
  set: (v: unknown) => setUiPref('uiStyle', normalizeUiChoice(v, 'panel', uiPrefChoices.uiStyle))
})
const uiStyleOptions = [
  { title: i18n.global.t('setting.uiStylePanel'), value: 'panel' },
  { title: i18n.global.t('setting.uiStyleGlass'), value: 'glass' },
  { title: i18n.global.t('setting.uiStyleSolid'), value: 'solid' },
  { title: i18n.global.t('setting.uiStyleClear'), value: 'clear' },
]
const uiDensityModel = computed({
  get: () => uiPrefs.value.uiDensity,
  set: (v: unknown) => setUiPref('uiDensity', normalizeUiChoice(v, 'comfortable', uiPrefChoices.uiDensity))
})
const uiDensityOptions = [
  { title: i18n.global.t('setting.uiDensityComfortable'), value: 'comfortable' },
  { title: i18n.global.t('setting.uiDensityCompact'), value: 'compact' },
]
const bgPresetModel = computed({
  get: () => uiPrefs.value.bgPreset,
  set: (v: unknown) => setUiPref('bgPreset', normalizeUiChoice(v, 'default', uiPrefChoices.bgPreset))
})
const bgPresetOptions = [
  { title: i18n.global.t('setting.bgPresetDefault'), value: 'default' },
  { title: i18n.global.t('setting.bgPresetNone'), value: 'none' },
  { title: i18n.global.t('setting.bgPresetCustom'), value: 'custom' },
]
const bgImageModel = computed({
  get: () => uiPrefs.value.bgImage,
  set: (v: string) => {
    setUiPref('bgImage', v)
    if (v) setUiPref('bgPreset', 'custom')
  }
})
const bgBlurModel = computed({
  get: () => parseInt(uiPrefs.value.bgBlur || '6'),
  set: (v: number) => setUiPref('bgBlur', String(v))
})
const bgOpacityModel = computed({
  get: () => parseInt(uiPrefs.value.bgOpacity || '40'),
  set: (v: number) => setUiPref('bgOpacity', String(v))
})
const bgSaturateModel = computed({
  get: () => Math.round(parseFloat(uiPrefs.value.bgSaturate || '1.3') * 100),
  set: (v: number) => setUiPref('bgSaturate', String(v / 100))
})
const bgFitModel = computed({
  get: () => uiPrefs.value.bgFit,
  set: (v: unknown) => setUiPref('bgFit', normalizeUiChoice(v, 'cover', uiPrefChoices.bgFit))
})
const bgFitOptions = [
  { title: i18n.global.t('setting.bgFitCover'), value: 'cover' },
  { title: i18n.global.t('setting.bgFitContain'), value: 'contain' },
  { title: i18n.global.t('setting.bgFitAuto'), value: 'auto' },
]
const bgPositionModel = computed({
  get: () => uiPrefs.value.bgPosition,
  set: (v: unknown) => setUiPref('bgPosition', normalizeUiChoice(v, 'center', uiPrefChoices.bgPosition))
})
const bgPositionOptions = [
  { title: i18n.global.t('setting.bgPositionCenter'), value: 'center' },
  { title: i18n.global.t('setting.bgPositionTop'), value: 'center top' },
  { title: i18n.global.t('setting.bgPositionBottom'), value: 'center bottom' },
]
const previewBackground = computed(() => {
  if (bgPresetModel.value === 'none') return ''
  if (bgPresetModel.value === 'custom') return bgImageModel.value
  return bgAsset
})
const handleBgFile = (value: File | File[] | undefined) => {
  const file = Array.isArray(value) ? value[0] : value
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => {
    bgImageModel.value = String(reader.result || '')
  }
  reader.readAsDataURL(file)
}
const resetUiPrefs = () => {
  ;['menuPosition', 'bgPreset', 'bgImage', 'bgBlur', 'bgOpacity', 'bgSaturate', 'bgFit', 'bgPosition', 'uiStyle', 'uiDensity'].forEach((key) => {
    localStorage.removeItem(key)
  })
  uiPrefs.value = readUiPrefs()
  notifyUiPrefs()
}
const rangeStyle = (value: number, min: number, max: number) => {
  const percent = Math.min(100, Math.max(0, ((value - min) / (max - min)) * 100))
  return { '--ui-range-percent': `${percent}%` }
}
const loading:Ref = inject('loading')?? ref(false)
const oldSettings = ref({})

const settings = ref({
	webListen: "",
	webDomain: "",
	webPort: "2095",
	webCertFile: "",
	webKeyFile: "",
  webPath: "/",
  webURI: "",
	sessionMaxAge: "0",
  trafficAge: "30",
  statsBucketSeconds: "60",
	timeLocation: "Asia/Shanghai",
  subListen: "",
	subPort: "2096",
	subPath: "/sub/",
	subDomain: "",
	subCertFile: "",
	subKeyFile: "",
	subUpdates: "12",
	subEncode: "true",
	subShowInfo: "false",
	subURI: "",
  subJsonExt: "",
  subClashExt: "",
  subClashNoDefGrp: "false",
  subClashSprtAll: "false",
  globalReset: "",
  congestionAlgo: "",
  qdisc: "",
  githubMirror: "",
  downloadLine: "auto",
})

const downloadLineOptions = computed(() => [
  { title: i18n.global.t('setting.downloadLineAuto'), value: 'auto' },
  { title: i18n.global.t('setting.downloadLineCN'), value: 'cn' },
  { title: i18n.global.t('setting.downloadLineGitHub'), value: 'github' },
])

const reverseProxyLoading = ref(false)
const reverseProxyStatus = ref<ReverseProxyStatus>(emptyReverseProxyStatus())
const reverseProxyConfig = ref({
  engine: 'caddy',
  domain: '',
})
const reverseProxyEngineOptions = [
  { title: 'Caddy', value: 'caddy' },
  { title: 'Nginx', value: 'nginx' },
]

const frontendLocation = new URL(resolveFrontendUrl())
const frontendAddress = frontendLocation.hostname
const frontendPort = frontendLocation.port || (frontendLocation.protocol === 'https:' ? '443' : '80')
const frontendPath = runtimeConfig.basePath
const backendApiURL = backendBaseUrl()

const reverseProxyUpstream = computed(() => {
  return reverseProxyStatus.value.apiListen || '-'
})

const reverseProxyPublicURL = computed(() => {
  const domain = reverseProxyConfig.value.domain.trim()
  if (domain) {
    const protocol = reverseProxyConfig.value.engine === 'caddy' ? 'https' : 'http'
    return `${protocol}://${domain}/`
  }
  return backendApiURL
})

const selectedReverseProxyInstalled = computed(() => {
  return reverseProxyConfig.value.engine === 'caddy'
    ? reverseProxyStatus.value.caddyInstalled
    : reverseProxyStatus.value.nginxInstalled
})

const canApplyReverseProxy = computed(() => {
  return reverseProxyStatus.value.supported &&
    selectedReverseProxyInstalled.value &&
    !(reverseProxyStatus.value.running && !reverseProxyStatus.value.managed)
})

onMounted(async () => {
  loading.value = true
  await loadData()
  await loadControllerMode()
  await loadXrayInstall()
  await loadReverseProxy()
  loading.value = false
  // Version checks use the GitHub API and must not delay the settings page.
  void loadVersion()
})

onBeforeUnmount(() => {
  if (versionPollTimer !== undefined) window.clearTimeout(versionPollTimer)
  if (xrayInstallPollTimer !== undefined) window.clearTimeout(xrayInstallPollTimer)
})

const formatMemory = (value: number) => value > 0 ? `${(value / (1024 ** 3)).toFixed(2)} GiB` : '-'

const loadControllerMode = async () => {
  controllerModeLoading.value = true
  await dataStore.loadControllerMode(true)
  controllerModeLoading.value = false
}

const setControllerMode = async (profile: string) => {
  if (profile === controllerMode.value.profile) return
  if (profile === 'client' && controllerMode.value.agent_count > 0) {
    const confirmed = window.confirm(i18n.global.t('setting.roleDisableConfirm', { count: controllerMode.value.agent_count }))
    if (!confirmed) return
  }
  controllerModeLoading.value = true
  pendingControllerProfile.value = profile
  const msg = await HttpUtils.post('api/controller-mode', { profile })
  if (msg.success && msg.obj) {
    dataStore.assignControllerMode(msg.obj)
    push.success({ message: i18n.global.t('setting.roleUpdated') })
    // A lite controller may have just turned Xray-core off.
    if (msg.obj.lite) await loadXrayInstall()
  }
  pendingControllerProfile.value = ''
  controllerModeLoading.value = false
}

const loadXrayInstall = async (): Promise<boolean> => {
  if (xrayInstallLoading.value) return false
  xrayInstallLoading.value = true
  const msg = await HttpUtils.get('api/xray-install')
  if (msg.success && msg.obj) {
    assignXrayInstallStatus(msg.obj)
  }
  xrayInstallLoading.value = false
  return msg.success
}

const pollXrayInstall = async () => {
  const loaded = await loadXrayInstall()
  if (!loaded || xrayInstallRunning.value) {
    xrayInstallPollTimer = window.setTimeout(pollXrayInstall, 1500)
    return
  }
  if (xrayInstall.value.install.state === 'success') {
    push.success({ message: i18n.global.t(xrayInstall.value.disabled ? 'setting.xrayInstallSuccessDisabled' : 'setting.xrayInstallSuccess') })
  }
}

const requestXrayInstall = async () => {
  const confirmed = window.confirm(i18n.global.t('setting.xrayInstallConfirm'))
  if (!confirmed) return
  xrayInstallLoading.value = true
  const msg = await HttpUtils.post('api/xray-install', {})
  xrayInstallLoading.value = false
  if (!msg.success) return
  if (msg.obj) {
    assignXrayInstallStatus(msg.obj)
  }
  await pollXrayInstall()
}

const assignXrayInstallStatus = (value: any) => {
  if (!value) return
  xrayInstall.value = {
    ...emptyXrayInstall(),
    ...value,
    install: { ...emptyXrayInstall().install, ...(value.install || {}) },
  }
}

const setXrayEnabled = async (enabled: boolean) => {
  if (!enabled) {
    const confirmed = window.confirm(i18n.global.t('setting.xrayDisableConfirm'))
    if (!confirmed) return
  }
  xrayInstallAction.value = 'enabled'
  const msg = await HttpUtils.post('api/xray-enabled', { enabled })
  if (msg.success) {
    assignXrayInstallStatus(msg.obj)
    push.success({ message: i18n.global.t(enabled ? 'setting.xrayEnabledSuccess' : 'setting.xrayDisabledSuccess') })
    await dataStore.loadControllerMode(true)
  }
  xrayInstallAction.value = ''
}

const startXray = async () => {
  if (xrayInstall.value.exclusive_run) {
    const confirmed = window.confirm(i18n.global.t('setting.xrayStartExclusiveConfirm'))
    if (!confirmed) return
  }
  xrayInstallAction.value = 'runtime'
  const msg = await HttpUtils.post('api/restartXray', {})
  if (msg.success) await loadXrayInstall()
  xrayInstallAction.value = ''
}

const stopXray = async () => {
  xrayInstallAction.value = 'runtime'
  const msg = await HttpUtils.post('api/stopXray', {})
  if (msg.success) await loadXrayInstall()
  xrayInstallAction.value = ''
}

const uninstallXray = async () => {
  if (xrayInstall.value.has_inbounds) return
  const confirmed = window.confirm(i18n.global.t('setting.xrayUninstallConfirm'))
  if (!confirmed) return
  xrayInstallAction.value = 'uninstall'
  const msg = await HttpUtils.post('api/xray-uninstall', {})
  if (msg.success) {
    assignXrayInstallStatus(msg.obj)
    push.success({ message: i18n.global.t('setting.xrayUninstalledSuccess') })
  }
  xrayInstallAction.value = ''
}

const openXrayInbounds = async () => {
  await router.push('/inbounds')
}

const loadData = async () => {
  loading.value = true
  const msg = await HttpUtils.get('api/settings')
  loading.value = false
  if (msg.success) {
    setData(msg.obj)
  }
}

const setData = (data: any) => {
  settings.value = data
  oldSettings.value = { ...data }
}

const loadReverseProxy = async () => {
  reverseProxyLoading.value = true
  const msg = await HttpUtils.get('api/reverse-proxy')
  if (msg.success && msg.obj) {
    reverseProxyStatus.value = { ...emptyReverseProxyStatus(), ...msg.obj }
    reverseProxyConfig.value = {
      engine: reverseProxyStatus.value.engine || 'caddy',
      domain: reverseProxyStatus.value.domain || '',
    }
  }
  reverseProxyLoading.value = false
}

const versionCheckFailed = ref(false)
// Install and lookup errors from GitHub say "lookup failed" or "download failed".
const githubFailure = /(lookup|download) failed/
const xrayGitHubFailed = computed(() => xrayInstall.value.install.state === 'failed'
  && githubFailure.test(xrayInstall.value.install.message || ''))
const versionGitHubFailed = computed(() => versionCheckFailed.value
  || (versionInfo.value.update.state === 'failed' && githubFailure.test(versionInfo.value.update.message || '')))

const loadVersion = async (): Promise<boolean> => {
  if (versionLoading.value) return false
  versionLoading.value = true
  const msg = await HttpUtils.get('api/version')
  // A failed check still reports the running version.
  if (msg.obj) {
    versionInfo.value = { ...emptyVersionInfo(), ...msg.obj, update: { ...emptyVersionInfo().update, ...(msg.obj.update || {}) } }
  }
  versionCheckFailed.value = !msg.success
  versionLoading.value = false
  return msg.success
}

const pollVersionUpdate = async () => {
  const loaded = await loadVersion()
  if (!loaded) {
    versionPollTimer = window.setTimeout(pollVersionUpdate, 2000)
    return
  }
  if (versionTarget.value && versionInfo.value.current === versionTarget.value) {
    await sleep(2500)
    window.location.reload()
    return
  }
  const state = versionInfo.value.update.state
  if (state === 'downloading' || state === 'installing' || state === 'restarting') {
    versionPollTimer = window.setTimeout(pollVersionUpdate, 2000)
    return
  }
  if (state === 'success') {
    await sleep(2500)
    window.location.reload()
    return
  }
  versionUpdating.value = false
}

const requestUpdate = async () => {
  if (!versionInfo.value.latest) return
  const confirmed = window.confirm(i18n.global.t('setting.versionUpdateConfirm', { version: versionInfo.value.latest }))
  if (!confirmed) return
  versionUpdating.value = true
  const msg = await HttpUtils.post('api/update', {})
  if (!msg.success) {
    versionUpdating.value = false
    return
  }
  if (msg.obj) versionInfo.value.update = { ...versionInfo.value.update, ...msg.obj }
  versionTarget.value = msg.obj?.version || versionInfo.value.latest
  await pollVersionUpdate()
}

const applyReverseProxy = async () => {
  reverseProxyLoading.value = true
  const msg = await HttpUtils.post('api/reverse-proxy', {
    engine: reverseProxyConfig.value.engine,
    domain: reverseProxyConfig.value.domain.trim(),
  })
  reverseProxyLoading.value = false
  if (!msg.success || !msg.obj) return

  reverseProxyStatus.value = { ...emptyReverseProxyStatus(), ...msg.obj }
  push.success({
    title: i18n.global.t('success'),
    duration: 2500,
    message: i18n.global.t('setting.reverseProxyApplied'),
  })

  await sleep(3200)
  window.location.replace(resolveFrontendUrl('settings'))
}

const save = async () => {
  loading.value = true
  const msg = await HttpUtils.post('api/save', { object: 'settings', action: 'set', data: JSON.stringify(settings.value) })
  if (msg.success) {
    push.success({
      title: i18n.global.t('success'),
      duration: 5000,
      message: i18n.global.t('actions.set') + " " + i18n.global.t('pages.settings')
    })
    setData(msg.obj.settings)
  }
  loading.value = false
}

const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))

const restartApp = async () => {
  loading.value = true
  const msg = await HttpUtils.post('api/restartApp',{})
  if (msg.success) {
    await sleep(3000)
    window.location.replace(resolveFrontendUrl('settings'))
  }
  loading.value = false
}

const subEncode = computed({
  get: () => { return settings.value.subEncode == "true" },
  set: (v:boolean) => { settings.value.subEncode = v ? "true" : "false" }
})

const subShowInfo = computed({
  get: () => { return settings.value.subShowInfo == "true" },
  set: (v:boolean) => { settings.value.subShowInfo = v ? "true" : "false" }
})

const webPort = computed({
  get: () => { return settings.value.webPort.length>0 ? parseInt(settings.value.webPort) : 2095 },
  set: (v:number) => { settings.value.webPort = v>0 ? v.toString() : "2095" }
})

const sessionMaxAge = computed({
  get: () => { return settings.value.sessionMaxAge.length>0 ? parseInt(settings.value.sessionMaxAge) : 0 },
  set: (v:number) => { settings.value.sessionMaxAge = v>0 ? v.toString() : "0" }
})

const trafficAge = computed({
  get: () => { return settings.value.trafficAge.length>0 ? parseInt(settings.value.trafficAge) : 0 },
  set: (v:number) => { settings.value.trafficAge = v>0 ? v.toString() : "0" }
})

const statsBucketSeconds = computed({
  get: () => { return settings.value.statsBucketSeconds.length>0 ? parseInt(settings.value.statsBucketSeconds) : 60 },
  set: (v:number) => { settings.value.statsBucketSeconds = v>0 ? v.toString() : "60" }
})

const subPort = computed({
  get: () => { return settings.value.subPort.length>0 ? parseInt(settings.value.subPort) : 2096 },
  set: (v:number) => { settings.value.subPort = v>0 ? v.toString() : "2096" }
})

const subUpdates = computed({
  get: () => { return settings.value.subUpdates.length>0 ? parseInt(settings.value.subUpdates) : 12 },
  set: (v:number) => { settings.value.subUpdates = v>0 ? v.toString() : "12" }
})

const stateChange = computed(() => {
  return !FindDiff.deepCompare(settings.value,oldSettings.value)
})

const sysctlLoading = ref(false)
const sysctlResult = ref('')
const sysctlError = ref(false)
const sysctlMessages = ref<string[]>([])

const bbrOptions = [
  { title: 'BBR v1', value: 'bbr' },
  { title: 'BBR v2', value: 'bbr2' },
  { title: 'BBR v3', value: 'bbr3' },
  { title: 'BBR v2 Plus', value: 'bbr2plus' },
  { title: 'BBR Plus', value: 'bbrplus' },
  { title: 'Cubic (默认)', value: 'cubic' },
]

const qdiscOptions = [
  { title: 'FQ (Fair Queue)', value: 'fq' },
  { title: 'CAKE', value: 'cake' },
  { title: '默认 (pfifo_fast)', value: '' },
]

const bbrVersion = computed({
  get: () => settings.value.congestionAlgo || 'bbr',
  set: (v: string) => { settings.value.congestionAlgo = v }
})

const qdisc = computed({
  get: () => settings.value.qdisc ?? '',
  set: (v: string) => { settings.value.qdisc = v }
})

const applyCongestion = async () => {
  sysctlLoading.value = true
  sysctlResult.value = ''
  sysctlError.value = false
  sysctlMessages.value = []

  const algo = bbrVersion.value || 'bbr'
  const qdiscVal = qdisc.value
  const msg = await HttpUtils.post('api/setSysctl', { congestionAlgo: algo, qdisc: qdiscVal })

  sysctlLoading.value = false
  if (msg.success) {
    sysctlResult.value = i18n.global.t('success')
    sysctlError.value = false
    sysctlMessages.value = msg.obj ?? []
    settings.value.congestionAlgo = algo
    settings.value.qdisc = qdiscVal
  } else {
    sysctlResult.value = i18n.global.t('failed') || 'Failed'
    sysctlError.value = true
    sysctlMessages.value = Array.isArray(msg.obj) ? msg.obj : (msg.msg ? msg.msg.split('\n') : [])
  }
}
</script>

<style scoped>
.role-section,
.xray-install-section,
.version-section {
  width: 100%;
}

.role-header,
.xray-install-header,
.version-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.role-capacity {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 18px;
  margin-top: 14px;
  color: rgba(var(--v-theme-on-surface), 0.68);
  font-size: 0.82rem;
}

.version-alert {
  width: 100%;
}

.xray-install-facts {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 14px;
  margin-top: 14px;
  color: rgba(var(--v-theme-on-surface), 0.68);
  font-size: 0.82rem;
}

.xray-install-status {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
}

.role-actions,
.xray-install-actions,
.version-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-top: 16px;
}

.reverse-proxy-section {
  width: 100%;
}

.reverse-proxy-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.reverse-proxy-status {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
}

.reverse-proxy-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 16px;
}

@media (max-width: 600px) {
  .role-header,
  .role-actions,
  .xray-install-header,
  .xray-install-actions,
  .version-header,
  .version-actions {
    align-items: stretch;
    flex-direction: column;
  }

  .role-header :deep(.v-btn),
  .role-actions :deep(.v-btn),
  .xray-install-header :deep(.v-btn),
  .xray-install-actions :deep(.v-btn),
  .version-header :deep(.v-btn),
  .version-actions :deep(.v-btn) {
    width: 100%;
  }

  .xray-install-status {
    justify-content: flex-start;
  }

  .reverse-proxy-header,
  .reverse-proxy-actions {
    align-items: stretch;
    flex-direction: column;
  }

  .reverse-proxy-status {
    justify-content: flex-start;
  }

  .reverse-proxy-actions :deep(.v-btn) {
    width: 100%;
  }
}

:deep(.ui-choice-field),
.ui-range-field {
  min-height: 64px;
  border: 1px solid rgba(var(--v-theme-on-surface), 0.08);
  border-radius: 10px;
  background: rgba(var(--v-theme-surface), 0.72);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
  padding: 8px;
}

:deep(.ui-choice-label),
.ui-range-header {
  color: rgba(var(--v-theme-on-surface), 0.68);
  font-size: 12px;
  line-height: 1.2;
  margin-bottom: 7px;
}

:deep(.ui-choice-group) {
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: minmax(0, 1fr);
  gap: 4px;
}

:deep(.ui-choice-button) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 0;
  height: 32px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  background-clip: padding-box;
  color: rgba(var(--v-theme-on-surface), 0.74);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.1;
  padding: 0 8px;
  transition: background 0.18s ease, color 0.18s ease, box-shadow 0.18s ease;
  white-space: nowrap;
  isolation: isolate;
  overflow: hidden;
  clip-path: inset(0 round 8px);
}

:deep(.ui-choice-button:hover) {
  background: rgba(var(--v-theme-primary), 0.08);
}

:deep(.ui-choice-button.is-active) {
  background: rgba(var(--v-theme-primary), 0.92);
  color: rgb(var(--v-theme-on-primary));
  box-shadow: 0 6px 16px rgba(var(--v-theme-primary), 0.22);
}

.ui-range-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 9px;
}

.ui-range-value {
  flex: 0 0 auto;
  min-width: 48px;
  border-radius: 999px;
  background: rgba(var(--v-theme-primary), 0.14);
  color: rgb(var(--v-theme-primary));
  font-size: 12px;
  font-weight: 700;
  padding: 3px 8px;
  text-align: center;
}

.ui-range {
  --ui-range-percent: 0%;
  display: block;
  width: 100%;
  height: 24px;
  margin: 0;
  appearance: none;
  background: transparent;
  cursor: pointer;
}

.ui-range::-webkit-slider-runnable-track {
  height: 6px;
  border-radius: 999px;
  background: linear-gradient(
    90deg,
    rgb(var(--v-theme-primary)) 0 var(--ui-range-percent),
    rgba(var(--v-theme-on-surface), 0.16) var(--ui-range-percent) 100%
  );
}

.ui-range::-webkit-slider-thumb {
  width: 18px;
  height: 18px;
  border: 3px solid rgb(var(--v-theme-surface));
  border-radius: 50%;
  appearance: none;
  background: rgb(var(--v-theme-primary));
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
  margin-top: -6px;
}

.ui-range::-moz-range-track {
  height: 6px;
  border-radius: 999px;
  background: rgba(var(--v-theme-on-surface), 0.16);
}

.ui-range::-moz-range-progress {
  height: 6px;
  border-radius: 999px;
  background: rgb(var(--v-theme-primary));
}

.ui-range::-moz-range-thumb {
  width: 14px;
  height: 14px;
  border: 3px solid rgb(var(--v-theme-surface));
  border-radius: 50%;
  background: rgb(var(--v-theme-primary));
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

.ui-range:focus-visible {
  outline: 2px solid rgba(var(--v-theme-primary), 0.35);
  outline-offset: 4px;
}

@media (max-width: 560px) {
  :deep(.ui-choice-group) {
    grid-auto-flow: row;
    grid-auto-columns: unset;
  }

  :deep(.ui-choice-button) {
    justify-content: center;
    white-space: normal;
  }
}
</style>
