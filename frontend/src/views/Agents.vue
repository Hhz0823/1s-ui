<template>
  <v-dialog v-model="enroll.visible" width="min(680px, calc(100vw - 24px))" scrollable>
    <v-card>
      <v-card-title class="text-center">{{ $t('agent.enroll') }}</v-card-title>
      <v-divider />
      <v-card-text>
        <v-alert type="info" variant="tonal" density="compact" class="mb-3">{{ $t('agent.note') }}</v-alert>
        <template v-if="!enroll.pairURL">
          <v-progress-linear v-if="enroll.apiLoading" indeterminate color="primary" class="mb-3" />
          <v-btn v-else color="primary" variant="tonal" block prepend-icon="mdi-refresh" @click="createEnrollmentAPI">
            {{ $t('agent.generateConnectionAPI') }}
          </v-btn>
        </template>
        <template v-else>
          <v-alert type="warning" variant="tonal" density="compact" class="mb-3">
            {{ $t('agent.pairSingleUse') }} {{ pairExpiryText }}
          </v-alert>
          <v-textarea :model-value="enroll.pairURL" :label="$t('agent.simpleAddress')" :hint="$t('agent.simpleAddressHint')" persistent-hint readonly dir="ltr" rows="2" auto-grow class="mb-3">
            <template #append-inner><v-btn icon="mdi-content-copy" size="small" variant="text" :title="$t('copyToClipboard')" @click.stop="copy(enroll.pairURL)" /></template>
          </v-textarea>
          <v-expansion-panels v-if="enroll.managedCommand" variant="accordion" class="enroll-commands">
            <v-expansion-panel>
              <v-expansion-panel-title>{{ $t('agent.installCommands') }}</v-expansion-panel-title>
              <v-expansion-panel-text>
                <v-textarea v-if="enroll.managedCommand" :model-value="enroll.managedCommand" :label="$t('agent.managedCommand')" readonly dir="ltr" rows="3" auto-grow hide-details class="mb-3">
                  <template #append-inner><v-btn icon="mdi-content-copy" size="small" variant="text" :title="$t('copyToClipboard')" @click.stop="copy(enroll.managedCommand)" /></template>
                </v-textarea>
                <v-textarea v-if="enroll.cnManagedCommand" :model-value="enroll.cnManagedCommand" :label="$t('agent.cnManagedCommand')" readonly dir="ltr" rows="3" auto-grow hide-details class="mb-3">
                  <template #append-inner><v-btn icon="mdi-content-copy" size="small" variant="text" :title="$t('copyToClipboard')" @click.stop="copy(enroll.cnManagedCommand)" /></template>
                </v-textarea>
              </v-expansion-panel-text>
            </v-expansion-panel>
          </v-expansion-panels>
        </template>
        <v-expansion-panels variant="accordion" class="enroll-commands mt-3">
          <v-expansion-panel>
            <v-expansion-panel-title>
              <v-icon icon="mdi-nas" size="small" class="mr-2" />{{ $t('agent.keyBind') }}
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <p class="text-body-2 mb-3">{{ $t('agent.keyBindHint') }}</p>
              <v-text-field :model-value="enrollKey.panel_url" :label="$t('agent.keyPanelUrl')" readonly dir="ltr" density="compact" class="mb-2" hide-details>
                <template #append-inner><v-btn icon="mdi-content-copy" size="small" variant="text" :title="$t('copyToClipboard')" @click.stop="copy(enrollKey.panel_url)" /></template>
              </v-text-field>
              <template v-if="enrollKey.key">
                <v-alert type="warning" variant="tonal" density="compact" class="my-2">{{ $t('agent.keyShownOnce') }}</v-alert>
                <v-text-field :model-value="enrollKey.key" :label="$t('agent.keyValue')" readonly dir="ltr" density="compact" class="mb-2" hide-details>
                  <template #append-inner><v-btn icon="mdi-content-copy" size="small" variant="text" :title="$t('copyToClipboard')" @click.stop="copy(enrollKey.key)" /></template>
                </v-text-field>
                <v-chip-group v-model="keyKind" mandatory column selected-class="text-primary" class="mt-1">
                  <v-chip v-for="kind in keyKinds" :key="kind.value" :value="kind.value" :prepend-icon="kind.icon" variant="outlined" filter>{{ $t(kind.title) }}</v-chip>
                </v-chip-group>
                <p class="text-body-2 text-medium-emphasis mb-3">{{ $t(keyKindHint) }}</p>
                <v-textarea v-for="item in keyCommands" :key="item.label" :model-value="item.value" :label="$t(item.label)" :hint="item.hint ? $t(item.hint) : ''" :persistent-hint="!!item.hint" :hide-details="!item.hint" readonly dir="ltr" rows="2" auto-grow class="mb-3">
                  <template #append-inner><v-btn icon="mdi-content-copy" size="small" variant="text" :title="$t('copyToClipboard')" @click.stop="copy(item.value)" /></template>
                </v-textarea>
              </template>
              <v-alert v-else-if="enrollKey.configured" type="info" variant="tonal" density="compact" class="my-2">{{ $t('agent.keyActive') }}</v-alert>
              <div class="d-flex flex-wrap ga-2 mt-2">
                <v-btn color="primary" variant="tonal" prepend-icon="mdi-key-plus" :loading="enrollKey.loading" @click="createEnrollmentKey">
                  {{ enrollKey.configured ? $t('agent.keyRegenerate') : $t('agent.keyGenerate') }}
                </v-btn>
                <v-btn v-if="enrollKey.configured" color="warning" variant="tonal" prepend-icon="mdi-key-remove" :loading="enrollKey.loading" @click="revokeEnrollmentKey">
                  {{ $t('agent.keyRevoke') }}
                </v-btn>
              </div>
            </v-expansion-panel-text>
          </v-expansion-panel>
        </v-expansion-panels>
      </v-card-text>
      <v-card-actions class="justify-center">
        <v-btn variant="outlined" @click="closeEnrollment">{{ $t('actions.close') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="connect.visible" width="min(580px, calc(100vw - 24px))">
    <v-card>
      <v-card-title class="text-center">{{ $t('agent.connectController') }}</v-card-title>
      <v-divider />
      <v-card-text>
        <v-alert v-if="localConnection.configured" type="success" variant="tonal" density="compact" class="mb-3">
          <strong>{{ $t('agent.localConnected') }}</strong>
          <div dir="ltr" class="mt-1">{{ localConnection.panel_url }}</div>
        </v-alert>
        <v-alert type="info" variant="tonal" density="compact" class="mb-3">{{ $t('agent.connectSimpleHint') }}</v-alert>
        <v-textarea v-model="connect.url" :label="$t('agent.connectInput')" :hint="$t('agent.connectInputHint')" persistent-hint rows="2" auto-grow dir="ltr" autofocus />
        <v-checkbox v-model="connect.insecure" :label="$t('agent.allowInsecure')" :hint="$t('agent.allowInsecureHint')" persistent-hint density="compact" hide-details="auto" />
      </v-card-text>
      <v-card-actions class="justify-center">
        <v-btn v-if="localConnection.configured" color="warning" variant="tonal" :loading="connect.loading" @click="disconnectController">{{ $t('agent.disconnectController') }}</v-btn>
        <v-btn variant="outlined" @click="connect.visible = false">{{ $t('actions.close') }}</v-btn>
        <v-btn color="primary" variant="tonal" prepend-icon="mdi-link-variant-plus" :loading="connect.loading" :disabled="!connect.url.trim()" @click="connectToController">{{ $t('agent.connectNow') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="edit.visible" width="min(640px, calc(100vw - 24px))" scrollable>
    <v-card>
      <v-card-title class="text-center">{{ $t('agent.editNode') }}</v-card-title>
      <v-divider />
      <v-card-text>
        <div class="edit-grid">
          <v-text-field v-model="edit.name" :label="$t('agent.name')" maxlength="80" hide-details class="span-2" />
          <v-text-field v-model="edit.publicHost" :label="$t('agent.publicHost')" :hint="$t('agent.publicHostHint')" persistent-hint dir="ltr" class="span-2" />
          <v-combobox v-model="edit.meta.group" :items="groups" :label="$t('monitor.group')" maxlength="40" hide-details />
          <v-text-field v-model="edit.meta.region" :label="$t('monitor.region')" :hint="$t('monitor.regionHint')" persistent-hint maxlength="2" dir="ltr">
            <template #prepend-inner><span class="flag-preview">{{ flagEmoji(edit.meta.region) }}</span></template>
          </v-text-field>
          <v-text-field v-model="edit.meta.tags" :label="$t('monitor.tags')" :hint="$t('monitor.tagsHint')" persistent-hint maxlength="255" class="span-2" />
          <v-text-field v-model.number="edit.meta.price" type="number" min="-1" step="0.01" :label="$t('monitor.price')" :hint="$t('monitor.priceHint')" persistent-hint />
          <div class="edit-pair">
            <v-text-field v-model="edit.meta.currency" :label="$t('monitor.currency')" maxlength="8" hide-details class="currency-field" />
            <v-select v-model="edit.meta.billing_cycle" :items="cycleOptions" item-title="title" item-value="value" :label="$t('monitor.billingCycle')" hide-details />
          </div>
          <v-text-field v-model="edit.expireDate" type="date" :label="$t('monitor.expireAt')" :hint="$t('monitor.expireHint')" persistent-hint clearable />
          <v-text-field v-model.number="edit.meta.sort_weight" type="number" :label="$t('monitor.sortWeight')" :hint="$t('monitor.sortWeightHint')" persistent-hint />
          <v-text-field v-model.number="edit.trafficLimitGiB" type="number" min="0" :label="$t('monitor.trafficLimit')" suffix="GiB" :hint="$t('monitor.trafficLimitHint')" persistent-hint />
          <div class="edit-pair">
            <v-select v-model="edit.meta.traffic_limit_type" :items="limitTypeOptions" item-title="title" item-value="value" :label="$t('monitor.limitType')" hide-details />
            <v-text-field v-model.number="edit.meta.traffic_reset_day" type="number" min="1" max="28" :label="$t('monitor.resetDay')" hide-details class="reset-field" />
          </div>
          <v-text-field v-model="edit.meta.remark" :label="$t('monitor.remark')" maxlength="255" hide-details class="span-2" />
        </div>
      </v-card-text>
      <v-card-actions class="justify-center">
        <v-btn variant="outlined" @click="edit.visible = false">{{ $t('actions.close') }}</v-btn>
        <v-btn color="primary" variant="tonal" :loading="edit.loading" :disabled="!edit.name.trim()" @click="saveNode">{{ $t('actions.save') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="batch.resultVisible" width="min(720px, calc(100vw - 24px))">
    <v-card>
      <v-card-title class="text-center">{{ $t('agent.batchResult') }}</v-card-title>
      <v-divider />
      <v-card-text>
        <v-list density="compact">
          <v-list-item v-for="item in batch.results" :key="item.node_id">
            <v-list-item-title>
              <v-chip size="x-small" :color="item.ok ? 'success' : 'error'" variant="tonal" class="me-2">{{ item.ok ? 'OK' : 'ERR' }}</v-chip>
              {{ item.name || ('#' + item.node_id) }}
            </v-list-item-title>
            <v-list-item-subtitle dir="ltr">{{ item.error || item.result?.output || item.result?.error || '-' }}</v-list-item-subtitle>
          </v-list-item>
        </v-list>
      </v-card-text>
      <v-card-actions class="justify-center"><v-btn variant="outlined" @click="batch.resultVisible = false">{{ $t('actions.close') }}</v-btn></v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="removeDialog.visible" width="min(420px, calc(100vw - 24px))">
    <v-card>
      <v-card-title class="text-center">{{ $t('actions.del') }}</v-card-title>
      <v-card-text class="text-center">{{ removeDialog.node?.name }}</v-card-text>
      <v-card-actions class="justify-center">
        <v-btn variant="outlined" @click="removeDialog.visible = false">{{ $t('no') }}</v-btn>
        <v-btn color="error" variant="tonal" :loading="removeDialog.loading" @click="deleteNode">{{ $t('yes') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <section class="monitor-page">
    <header class="monitor-heading">
      <div>
        <h1 class="adaptive-ink">{{ $t('pages.agents') }}</h1>
        <p class="adaptive-ink">{{ $t('agent.overviewHint') }}</p>
      </div>
      <div class="heading-actions">
        <v-chip v-if="localConnection.configured" color="success" prepend-icon="mdi-link-variant" size="small">
          {{ $t('agent.localConnected') }}
        </v-chip>
        <v-btn variant="tonal" prepend-icon="mdi-link-variant-plus" @click="openLocalConnect">{{ $t('agent.connectController') }}</v-btn>
        <v-btn
          v-if="controllerMode.profile === 'client'"
          color="primary"
          prepend-icon="mdi-server-network"
          :loading="controllerModeLoading"
          :disabled="!controllerMode.can_enable"
          :title="controllerMode.can_enable ? '' : serverRequirementText"
          @click="enableControllerMode"
        >{{ $t('setting.roleEnable') }}</v-btn>
        <v-btn v-else color="primary" prepend-icon="mdi-server-plus" :disabled="!serverMonitoringAvailable" :title="serverMonitoringAvailable ? '' : serverRequirementText" @click="openEnrollment">{{ $t('agent.enroll') }}</v-btn>
        <v-btn icon="mdi-refresh" variant="tonal" :loading="loading" :title="$t('actions.update')" @click="loadNodes" />
      </div>
    </header>

    <v-alert v-if="!controllerMode.enabled" type="info" variant="tonal" density="comfortable" border="start" icon="mdi-server-outline">
      {{ $t('agent.controllerDisabledHint') }}
    </v-alert>
    <v-alert v-else-if="hostRequirements && !serverMonitoringAvailable" type="error" variant="tonal" density="comfortable" border="start" icon="mdi-server-off">
      {{ serverRequirementText }}
    </v-alert>

    <section class="stat-card">
      <v-menu :close-on-content-click="false" location="bottom end">
        <template #activator="{ props }">
          <v-btn v-bind="props" icon="mdi-cog-outline" size="x-small" variant="text" class="stat-settings" :title="$t('monitor.statusSettings')" />
        </template>
        <v-card min-width="260" class="pa-3">
          <div class="text-subtitle-2 mb-2">{{ $t('monitor.statusSettings') }}</div>
          <v-switch v-for="item in statDefs" :key="item.key" v-model="statVisible[item.key]" :label="item.title" density="compact" hide-details color="primary" inset @update:model-value="saveStatVisible" />
        </v-card>
      </v-menu>
      <div class="stat-grid">
        <div v-for="item in shownStats" :key="item.key" class="stat-item">
          <span>{{ item.title }}</span>
          <strong dir="ltr">{{ item.value }}</strong>
        </div>
      </div>
    </section>

    <div class="control-bar">
      <v-text-field
        ref="searchField"
        v-model="query"
        prepend-inner-icon="mdi-magnify"
        :placeholder="$t('monitor.searchPlaceholder')"
        density="compact"
        hide-details
        clearable
        class="search-field"
        @keydown.esc="query = ''"
      />
      <div class="control-right">
        <v-select v-model="sortKey" :items="sortOptions" item-title="title" item-value="value" density="compact" hide-details class="sort-field" />
        <v-btn :icon="sortDesc ? 'mdi-sort-descending' : 'mdi-sort-ascending'" variant="tonal" size="small" :title="$t('agent.sortDirection')" @click="sortDesc = !sortDesc" />
        <span class="control-label">{{ $t('monitor.viewMode') }}</span>
        <v-btn icon="mdi-view-grid-outline" size="small" :variant="viewMode === 'grid' ? 'flat' : 'tonal'" :color="viewMode === 'grid' ? 'primary' : undefined" :title="$t('monitor.viewGrid')" @click="setViewMode('grid')" />
        <v-btn icon="mdi-table" size="small" :variant="viewMode === 'table' ? 'flat' : 'tonal'" :color="viewMode === 'table' ? 'primary' : undefined" :title="$t('monitor.viewTable')" @click="setViewMode('table')" />
      </div>
    </div>

    <div v-if="groups.length" class="group-bar">
      <span class="control-label">{{ $t('monitor.group') }}</span>
      <v-btn-toggle v-model="selectedGroup" mandatory density="compact" variant="outlined" divided color="primary" class="group-toggle" @update:model-value="saveGroup">
        <v-btn value="all" size="small">{{ $t('monitor.all') }}</v-btn>
        <v-btn v-for="group in groups" :key="group" :value="group" size="small">{{ group }}</v-btn>
      </v-btn-toggle>
    </div>

    <div class="list-summary">
      <span>{{ summaryText }}</span>
      <template v-if="canControl">
        <v-btn size="small" variant="text" prepend-icon="mdi-checkbox-multiple-marked" @click="selectAllControllable">{{ $t('agent.selectOnline') }}</v-btn>
        <v-btn v-if="selected.length" size="small" variant="text" @click="selected = []">{{ $t('agent.clearSelection') }}</v-btn>
      </template>
    </div>

    <div v-if="canControl && selected.length" class="batch-bar">
      <strong>{{ $t('agent.batchSelected', { n: selected.length }) }}</strong>
      <v-btn size="small" variant="tonal" :loading="batch.loading" @click="batchCmd('report_now')">{{ $t('agent.cmdReportNow') }}</v-btn>
      <v-btn size="small" variant="tonal" :loading="batch.loading" @click="batchCmd('ping')">{{ $t('agent.cmdPing') }}</v-btn>
      <v-btn size="small" variant="tonal" color="warning" :loading="batch.loading" @click="batchCmd('restart_singbox')">{{ $t('agent.cmdRestartSingBox') }}</v-btn>
      <v-btn size="small" variant="tonal" color="warning" :loading="batch.loading" @click="batchCmd('restart_xray')">{{ $t('agent.cmdRestartXray') }}</v-btn>
      <v-text-field v-model="batch.shell" density="compact" hide-details :placeholder="$t('agent.execPlaceholder')" class="batch-shell" dir="ltr" />
      <v-btn size="small" color="primary" variant="tonal" :loading="batch.loading" :disabled="!batch.shell.trim()" @click="batchCmd('exec', { command: batch.shell.trim() })">{{ $t('agent.cmdExec') }}</v-btn>
    </div>

    <v-progress-linear v-if="loading && !nodes.length" indeterminate />
    <div v-if="!loading && filteredNodes.length === 0" class="empty-state">
      <div>{{ nodes.length ? $t('agent.noMatch') : $t('agent.noNodes') }}</div>
      <small v-if="nodes.length">{{ $t('monitor.tryDifferent') }}</small>
    </div>

    <div v-else-if="viewMode === 'grid'" class="node-grid">
      <article v-for="node in filteredNodes" :key="node.id" class="node-card" :class="{ 'node-card--offline': !node.online }" @click="openDetail(node)">
        <header class="node-card__head">
          <div class="node-card__identity">
            <span v-if="flagEmoji(node.region)" class="node-flag">{{ flagEmoji(node.region) }}</span>
            <div class="node-card__title">
              <strong>{{ node.name }}</strong>
              <small class="mobile-only">{{ node.online ? uptimeText(node.report.uptime, t) : '-' }}</small>
              <PriceTags :node="node" class="desktop-only" />
            </div>
          </div>
          <div class="node-card__status" @click.stop>
            <v-checkbox-btn v-if="canControl && node.controllable" density="compact" :model-value="selected.includes(node.id)" @update:model-value="toggleSelect(node.id, $event)" />
            <span class="status-badge" :class="node.online ? 'status-badge--on' : 'status-badge--off'">{{ node.online ? $t('online') : $t('agent.offline') }}</span>
            <v-menu location="bottom end">
              <template #activator="{ props }"><v-btn v-bind="props" icon="mdi-dots-vertical" size="x-small" variant="text" :title="$t('actions.action')" /></template>
              <v-list density="compact">
                <v-list-item prepend-icon="mdi-information-outline" :title="$t('agent.detail')" @click="openDetail(node)" />
                <template v-if="canControl">
                  <v-list-item prepend-icon="mdi-tune-vertical" :title="$t('agent.manageInbounds')" :disabled="!node.managed" @click="manageInbounds(node)" />
                  <v-list-item prepend-icon="mdi-pencil-outline" :title="$t('agent.editNode')" @click="openEdit(node)" />
                  <v-list-item prepend-icon="mdi-key-change" :title="$t('agent.rotate')" @click="rotateNode(node)" />
                  <v-list-item prepend-icon="mdi-delete-outline" :title="$t('actions.del')" base-color="error" @click="askDelete(node)" />
                </template>
              </v-list>
            </v-menu>
          </div>
        </header>
        <v-divider />
        <div class="node-card__body">
          <div class="info-row desktop-only">
            <span>{{ $t('monitor.os') }}</span>
            <strong><v-icon :icon="osIcon(node)" size="18" class="me-1" />{{ osName(node) || '-' }}<template v-if="node.report.arch"> / {{ node.report.arch }}</template></strong>
          </div>
          <div class="usage-stack">
            <UsageBar label="CPU" :value="node.online ? node.report.cpu_percent : undefined" :detail="node.report.cpu_cores ? $t('monitor.coresN', { n: node.report.cpu_cores }) : ''" />
            <UsageBar :label="$t('monitor.ram')" :value="node.online ? usagePercent(node.report.memory) : undefined" :detail="usageText(node.report.memory)" />
            <UsageBar :label="$t('agent.disk')" :value="node.online ? usagePercent(node.report.disk) : undefined" :detail="usageText(node.report.disk)" />
          </div>
          <template v-if="node.traffic_limit">
            <UsageBar :label="$t('monitor.periodTraffic')" :value="trafficPercent(node)" />
            <div class="info-row info-row--sub">
              <span dir="ltr">↑ {{ bytes(node.traffic?.sent) }} ↓ {{ bytes(node.traffic?.recv) }}</span>
              <span dir="ltr">{{ limitTypeLabel(node.traffic_limit_type) }}({{ bytes(node.traffic_limit) }})</span>
            </div>
          </template>
          <div v-else class="info-row">
            <span>{{ $t('monitor.totalTraffic') }}</span>
            <strong dir="ltr">↑ {{ bytes(node.report.network?.sent) }} ↓ {{ bytes(node.report.network?.recv) }}</strong>
          </div>
          <div class="info-row">
            <span>{{ $t('monitor.networkSpeed') }}</span>
            <strong dir="ltr">↑ {{ node.online ? rate(node.report.net_rate?.sent) : '-' }} ↓ {{ node.online ? rate(node.report.net_rate?.recv) : '-' }}</strong>
          </div>
          <div class="info-row">
            <span>{{ $t('agent.uptime') }}</span>
            <strong :class="{ 'text-medium-emphasis': !node.online }">{{ node.online ? uptimeText(node.report.uptime, t) : '-' }}</strong>
          </div>
          <div class="info-row">
            <span>{{ $t('monitor.loadConns') }}</span>
            <strong dir="ltr">{{ node.online ? loadText(node) : '-' }}</strong>
          </div>
          <div class="info-row">
            <span>{{ $t('monitor.pingProxy') }}</span>
            <strong class="core-badges">
              <span dir="ltr" :class="latencyClass(node)">{{ latencyLabel(node) }}</span>
              <span class="core-badge" :class="{ 'core-badge--on': node.online && node.report.cores?.singbox_running }">sing-box</span>
              <span class="core-badge" :class="{ 'core-badge--on': node.online && node.report.cores?.xray_running }">Xray</span>
            </strong>
          </div>
          <PriceTags :node="node" class="mobile-only" />
        </div>
      </article>
    </div>

    <div v-else class="node-table-wrap">
      <table class="node-table">
        <thead>
          <tr>
            <th class="col-expand" />
            <th v-for="column in tableColumns" :key="column.key" :class="column.class" @click="sortBy(column.key)">
              <span>{{ column.title }}</span>
              <v-icon v-if="sortKey === column.key" :icon="sortDesc ? 'mdi-chevron-down' : 'mdi-chevron-up'" size="14" />
            </th>
            <th v-if="canControl" class="col-menu" />
          </tr>
        </thead>
        <tbody>
          <template v-for="node in filteredNodes" :key="node.id">
            <tr class="node-row" :class="{ 'node-row--open': expanded.includes(node.id), 'node-row--offline': !node.online }" @click="toggleExpand(node.id)">
              <td class="col-expand"><v-icon :icon="expanded.includes(node.id) ? 'mdi-chevron-down' : 'mdi-chevron-right'" size="18" /></td>
              <td class="col-name">
                <div class="table-name">
                  <span v-if="flagEmoji(node.region)" class="node-flag">{{ flagEmoji(node.region) }}</span>
                  <div>
                    <strong>{{ node.name }}</strong>
                    <small>{{ node.online ? uptimeText(node.report.uptime, t) : '-' }}</small>
                  </div>
                </div>
              </td>
              <td><v-icon :icon="osIcon(node)" size="18" :title="osName(node)" /></td>
              <td><span class="status-badge" :class="node.online ? 'status-badge--on' : 'status-badge--off'">{{ node.online ? $t('online') : $t('agent.offline') }}</span></td>
              <td class="col-bar"><UsageBar compact :value="node.online ? node.report.cpu_percent : undefined" /></td>
              <td class="col-bar"><UsageBar compact :value="node.online ? usagePercent(node.report.memory) : undefined" /></td>
              <td class="col-bar"><UsageBar compact :value="node.online ? usagePercent(node.report.disk) : undefined" /></td>
              <td class="col-tags"><PriceTags :node="node" :show-ip="false" /></td>
              <td dir="ltr">↑{{ node.online ? rate(node.report.net_rate?.sent) : '-' }}</td>
              <td dir="ltr">↓{{ node.online ? rate(node.report.net_rate?.recv) : '-' }}</td>
              <td dir="ltr">↑{{ bytes(node.report.network?.sent) }}</td>
              <td dir="ltr">↓{{ bytes(node.report.network?.recv) }}</td>
              <td dir="ltr">{{ bytes(trafficUsed(node)) }}<template v-if="node.traffic_limit"> / {{ bytes(node.traffic_limit) }}</template></td>
              <td dir="ltr" :class="latencyClass(node)">{{ latencyLabel(node) }}</td>
              <td v-if="canControl" class="col-menu" @click.stop>
                <v-menu location="bottom end">
                  <template #activator="{ props }"><v-btn v-bind="props" icon="mdi-dots-vertical" size="x-small" variant="text" /></template>
                  <v-list density="compact">
                    <v-list-item prepend-icon="mdi-tune-vertical" :title="$t('agent.manageInbounds')" :disabled="!node.managed" @click="manageInbounds(node)" />
                    <v-list-item prepend-icon="mdi-pencil-outline" :title="$t('agent.editNode')" @click="openEdit(node)" />
                    <v-list-item prepend-icon="mdi-key-change" :title="$t('agent.rotate')" @click="rotateNode(node)" />
                    <v-list-item prepend-icon="mdi-delete-outline" :title="$t('actions.del')" base-color="error" @click="askDelete(node)" />
                  </v-list>
                </v-menu>
              </td>
            </tr>
            <tr v-if="expanded.includes(node.id)" class="node-expand">
              <td :colspan="tableColumns.length + (canControl ? 2 : 1)">
                <div class="expand-grid">
                  <div v-for="item in nodeFacts(node)" :key="item.label"><span>{{ item.label }}</span><strong dir="ltr">{{ item.value }}</strong></div>
                </div>
                <div class="expand-actions">
                  <v-btn size="small" variant="tonal" color="primary" prepend-icon="mdi-chart-line" @click="openDetail(node)">{{ $t('agent.detail') }}</v-btn>
                </div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
  </section>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { push } from 'notivue'
import { i18n } from '@/locales'
import Data from '@/store/modules/data'
import type { AgentNode, AgentNodeMeta, TrafficLimitType } from '@/types/agents'
import { copyText } from '@/utils/clipboard'
import { fetchBackendObject as api, resolveFrontendUrl } from '@/utils/backend'
import UsageBar from '@/components/monitor/UsageBar.vue'
import PriceTags from '@/components/monitor/PriceTags.vue'
import {
  billingCycles, bytes, cycleKey, flagEmoji, osIcon, osName, rate, trafficPercent, trafficUsed, uptimeText, usagePercent, usageText,
} from '@/utils/monitor'

const { t } = useI18n()
const router = useRouter()
const dataStore = Data()
const nodes = ref<AgentNode[]>([])
const loading = ref(false)
const query = ref('')
const sortKey = ref('default')
const sortDesc = ref(false)
const selected = ref<number[]>([])
const expanded = ref<number[]>([])
const searchField = ref<any>(null)
const now = ref(new Date())
const hostRequirements = computed(() => dataStore.hostRequirements)
const controllerModeLoading = ref(false)
const controllerMode = computed(() => dataStore.controllerMode)
const canControl = computed(() => controllerMode.value.can_control !== false)
const localConnection = reactive({ supported: false, installed: false, configured: false, running: false, panel_url: '', public_url: '', insecure: false })
const serverMonitoringAvailable = computed(() => controllerMode.value.enabled && hostRequirements.value?.can_enable_agents === true)
const currentHostMemoryGiB = computed(() => {
  const value = Number(hostRequirements.value?.mem_total_bytes || 0)
  return value > 0 ? (value / (1024 ** 3)).toFixed(2) : '?'
})
const serverRequirementText = computed(() => i18n.global.t('agent.hostRequirement', {
  cpu: hostRequirements.value?.cpu_cores ?? '?',
  memory: currentHostMemoryGiB.value,
}))

// View preferences are per browser.
const readPref = (key: string, fallback: string) => {
  try { return localStorage.getItem(key) || fallback } catch { return fallback }
}
const writePref = (key: string, value: string) => {
  try { localStorage.setItem(key, value) } catch { /* private mode */ }
}
const viewMode = ref<'grid' | 'table'>(readPref('monitorViewMode', 'grid') === 'table' ? 'table' : 'grid')
const selectedGroup = ref(readPref('monitorGroup', 'all'))
const setViewMode = (mode: 'grid' | 'table') => { viewMode.value = mode; writePref('monitorViewMode', mode) }
const saveGroup = () => writePref('monitorGroup', selectedGroup.value)

const defaultMeta = (): AgentNodeMeta => ({
  group: '', tags: '', region: '', remark: '', price: 0, currency: '', billing_cycle: 30, expire_at: 0,
  sort_weight: 0, traffic_limit: 0, traffic_limit_type: 'sum', traffic_reset_day: 1,
})
const enroll = reactive({ visible: false, apiLoading: false, pairURL: '', pairExpiresAt: 0, command: '', managedCommand: '', cnManagedCommand: '' })
const emptyKeyCommands = () => ({
  command: '', nas_command: '', cn_command: '', cn_nas_command: '',
  client_command: '', client_nas_command: '', cn_client_command: '', cn_client_nas_command: '',
  openwrt_command: '', cn_openwrt_command: '',
})
type KeyCommandName = keyof ReturnType<typeof emptyKeyCommands>
const enrollKey = reactive({ loading: false, configured: false, panel_url: '', key: '', ...emptyKeyCommands() })
// One key binds three kinds of device: the full 1S-UI as a client (fnOS or
// another Linux box), the one-process OpenWrt build, or only the agent.
type KeyKind = 'client' | 'openwrt' | 'agent'
const keyKind = ref<KeyKind>('client')
const keyKinds: { value: KeyKind, title: string, icon: string }[] = [
  { value: 'client', title: 'agent.keyKindClient', icon: 'mdi-nas' },
  { value: 'openwrt', title: 'agent.keyKindOpenwrt', icon: 'mdi-router-wireless' },
  { value: 'agent', title: 'agent.keyKindAgent', icon: 'mdi-server' },
]
const keyKindHint = computed(() => ({ client: 'agent.keyClientHint', openwrt: 'agent.keyOpenwrtHint', agent: 'agent.keyAgentHint' })[keyKind.value])
const keyCommandList: Record<KeyKind, { name: KeyCommandName, label: string, hint?: string }[]> = {
  client: [
    { name: 'client_nas_command', label: 'agent.keyNasCommand', hint: 'agent.keyNasCommandHint' },
    { name: 'client_command', label: 'agent.keyRootCommand' },
    { name: 'cn_client_nas_command', label: 'agent.keyCnNasCommand' },
    { name: 'cn_client_command', label: 'agent.keyCnRootCommand' },
  ],
  openwrt: [
    { name: 'openwrt_command', label: 'agent.keyOpenwrtCommand', hint: 'agent.keyOpenwrtCommandHint' },
    { name: 'cn_openwrt_command', label: 'agent.keyCnOpenwrtCommand' },
  ],
  agent: [
    { name: 'nas_command', label: 'agent.keyNasCommand', hint: 'agent.keyNasCommandHint' },
    { name: 'command', label: 'agent.keyRootCommand' },
    { name: 'cn_nas_command', label: 'agent.keyCnNasCommand' },
    { name: 'cn_command', label: 'agent.keyCnRootCommand' },
  ],
}
const keyCommands = computed(() => keyCommandList[keyKind.value]
  .map(item => ({ ...item, value: enrollKey[item.name] }))
  .filter(item => item.value))
const connect = reactive({ visible: false, loading: false, url: '', insecure: false })
const edit = reactive({ visible: false, loading: false, id: 0, name: '', publicHost: '', meta: defaultMeta(), expireDate: '', trafficLimitGiB: 0 })
const removeDialog = reactive<{ visible: boolean, loading: boolean, node?: AgentNode }>({ visible: false, loading: false })
const batch = reactive<{ loading: boolean, shell: string, results: any[], resultVisible: boolean }>({ loading: false, shell: '', results: [], resultVisible: false })
let refreshTimer: number | undefined
let clockTimer: number | undefined

const pairExpiryText = computed(() => enroll.pairExpiresAt
  ? i18n.global.t('agent.pairExpires', { time: new Date(enroll.pairExpiresAt * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) })
  : '')

const groups = computed(() => [...new Set(nodes.value.map(node => node.group || '').filter(Boolean))].sort((a, b) => a.localeCompare(b)))
const onlineNodes = computed(() => nodes.value.filter(node => node.online))
const sum = (list: AgentNode[], pick: (node: AgentNode) => number | undefined) => list.reduce((total, node) => total + Number(pick(node) || 0), 0)

// Summary stats: the first five match the Komari layout, the rest are ours.
const statDefs = computed(() => {
  const online = onlineNodes.value
  const regions = new Set(online.map(node => node.region).filter(Boolean)).size
  const cores = online.filter(node => node.report.cores?.singbox_running || node.report.cores?.xray_running).length
  const alerts = nodes.value.filter(node => needsAttention(node)).length
  return [
    { key: 'time', title: t('monitor.currentTime'), value: now.value.toLocaleTimeString() },
    { key: 'online', title: t('monitor.currentOnline'), value: `${online.length} / ${nodes.value.length}` },
    { key: 'regions', title: t('monitor.regionOverview'), value: String(regions) },
    { key: 'traffic', title: t('monitor.trafficOverview'), value: `↑ ${bytes(sum(online, node => node.report.network?.sent))} / ↓ ${bytes(sum(online, node => node.report.network?.recv))}` },
    { key: 'speed', title: t('monitor.networkSpeed'), value: `↑ ${rate(sum(online, node => node.report.net_rate?.sent))} / ↓ ${rate(sum(online, node => node.report.net_rate?.recv))}` },
    { key: 'period', title: t('monitor.periodTraffic'), value: `↑ ${bytes(sum(nodes.value, node => node.traffic?.sent))} / ↓ ${bytes(sum(nodes.value, node => node.traffic?.recv))}` },
    { key: 'cores', title: t('monitor.proxyCores'), value: `${cores} / ${online.length}` },
    { key: 'alerts', title: t('monitor.attention'), value: String(alerts) },
  ]
})
const statVisible = reactive<Record<string, boolean>>((() => {
  try { return JSON.parse(localStorage.getItem('monitorStatVisible') || '{}') } catch { return {} }
})())
const saveStatVisible = () => writePref('monitorStatVisible', JSON.stringify(statVisible))
const shownStats = computed(() => statDefs.value.filter(item => statVisible[item.key] !== false))

// needsAttention flags offline servers, high usage, nearly used traffic and
// servers that expire within a week.
const needsAttention = (node: AgentNode) => {
  if (!node.online) return true
  if (Number(node.report.cpu_percent || 0) >= 90 || Number(usagePercent(node.report.memory) || 0) >= 90 || Number(usagePercent(node.report.disk) || 0) >= 90) return true
  if (Number(trafficPercent(node) || 0) >= 90) return true
  return Boolean(node.expire_at && node.expire_at * 1000 - Date.now() < 7 * 86400000)
}

const sortOptions = computed(() => [
  { title: i18n.global.t('agent.sortDefault'), value: 'default' },
  { title: i18n.global.t('agent.name'), value: 'name' },
  { title: 'CPU', value: 'cpu' },
  { title: i18n.global.t('agent.memory'), value: 'memory' },
  { title: i18n.global.t('agent.disk'), value: 'disk' },
  { title: i18n.global.t('agent.latency'), value: 'latency' },
  { title: t('monitor.upSpeed'), value: 'upload' },
  { title: t('monitor.downSpeed'), value: 'download' },
  { title: t('monitor.periodTraffic'), value: 'period' },
  { title: t('monitor.expireAt'), value: 'expire' },
])
const tableColumns = computed(() => [
  { key: 'name', title: t('agent.name'), class: 'col-name' },
  { key: 'os', title: t('monitor.os'), class: '' },
  { key: 'status', title: t('agent.status'), class: '' },
  { key: 'cpu', title: 'CPU', class: 'col-bar' },
  { key: 'memory', title: t('monitor.ram'), class: 'col-bar' },
  { key: 'disk', title: t('agent.disk'), class: 'col-bar' },
  { key: 'price', title: t('monitor.price'), class: 'col-tags' },
  { key: 'upload', title: t('monitor.upSpeed'), class: '' },
  { key: 'download', title: t('monitor.downSpeed'), class: '' },
  { key: 'totalUp', title: t('monitor.totalUp'), class: '' },
  { key: 'totalDown', title: t('monitor.totalDown'), class: '' },
  { key: 'period', title: t('monitor.periodTraffic'), class: '' },
  { key: 'latency', title: 'Ping', class: '' },
])
// Table headers cycle default, ascending, descending.
const sortBy = (key: string) => {
  if (sortKey.value !== key) { sortKey.value = key; sortDesc.value = false; return }
  if (!sortDesc.value) { sortDesc.value = true; return }
  sortKey.value = 'default'
  sortDesc.value = false
}

const filteredNodes = computed(() => {
  const needle = query.value?.trim().toLowerCase() || ''
  const status = ['online', '在线', '在線'].includes(needle) ? true : ['offline', '离线', '離線'].includes(needle) ? false : undefined
  const result = nodes.value.filter(node => {
    if (selectedGroup.value !== 'all' && (node.group || '') !== selectedGroup.value) return false
    if (!needle) return true
    if (status !== undefined) return node.online === status
    return [node.name, node.report.hostname, node.remote_ip, node.public_host, node.report.os, node.report.platform, node.report.arch,
      node.region, flagEmoji(node.region), node.group, node.tags, node.remark, node.price ? String(node.price) : '']
      .some(value => String(value || '').toLowerCase().includes(needle))
  })
  const value = (node: AgentNode): number | string => {
    switch (sortKey.value) {
      case 'name': return node.name.toLowerCase()
      case 'os': return osName(node).toLowerCase()
      case 'status': return node.online ? 1 : 0
      case 'cpu': return Number(node.report.cpu_percent || 0)
      case 'memory': return Number(usagePercent(node.report.memory) || 0)
      case 'disk': return Number(usagePercent(node.report.disk) || 0)
      case 'latency': return Number(node.latency?.last_ms ?? Number.MAX_SAFE_INTEGER)
      case 'upload': return Number(node.report.net_rate?.sent || 0)
      case 'download': return Number(node.report.net_rate?.recv || 0)
      case 'totalUp': return Number(node.report.network?.sent || 0)
      case 'totalDown': return Number(node.report.network?.recv || 0)
      case 'period': return trafficUsed(node)
      case 'price': return Number(node.price || 0)
      case 'expire': return node.expire_at || Number.MAX_SAFE_INTEGER
      default: return Number(node.sort_weight || 0) * 1e9 + node.id
    }
  }
  return result.slice().sort((a, b) => {
    // Offline servers go last in the default order.
    if (sortKey.value === 'default' && a.online !== b.online) return a.online ? -1 : 1
    const av = value(a)
    const bv = value(b)
    const order = typeof av === 'string' && typeof bv === 'string' ? av.localeCompare(bv) : Number(av) - Number(bv)
    return sortDesc.value ? -order : order
  })
})
const summaryText = computed(() => {
  if (query.value?.trim()) return t('monitor.searchResults', { n: filteredNodes.value.length })
  const list = selectedGroup.value === 'all' ? nodes.value : filteredNodes.value
  return t('monitor.totalNodes', { total: list.length, online: list.filter(node => node.online).length })
})

const cycleOptions = computed(() => billingCycles.map(days => ({ value: days, title: t(`monitor.cycle.${cycleKey(days)}`) })))
const limitTypeOptions = computed(() => (['sum', 'max', 'min', 'up', 'down'] as TrafficLimitType[]).map(value => ({ value, title: t(`monitor.limit.${value}`) })))
const limitTypeLabel = (value?: string) => t(`monitor.limit.${value || 'sum'}`)

const loadNodes = async () => {
  if (loading.value) return
  loading.value = true
  try {
    nodes.value = await api('api/agents') || []
    selected.value = canControl.value ? selected.value.filter(id => nodes.value.some(node => node.id === id && node.controllable)) : []
    if (selectedGroup.value !== 'all' && !groups.value.includes(selectedGroup.value)) selectedGroup.value = 'all'
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('agent.loadFailed') })
  } finally { loading.value = false }
}
const handleVisibilityChange = () => { if (!document.hidden) void loadNodes() }
// "/" focuses the search box, like the Komari home page.
const handleKey = (event: KeyboardEvent) => {
  const target = event.target as HTMLElement | null
  if (event.key !== '/' || target?.closest('input, textarea, [contenteditable="true"]')) return
  event.preventDefault()
  searchField.value?.focus?.()
}

const openEnrollment = () => {
  if (!controllerMode.value.enabled) return push.error({ message: i18n.global.t('agent.controllerDisabledHint') })
  if (!serverMonitoringAvailable.value) return push.error({ message: serverRequirementText.value })
  Object.assign(enroll, { visible: true, apiLoading: false, pairURL: '', pairExpiresAt: 0, command: '', managedCommand: '', cnManagedCommand: '' })
  void createEnrollmentAPI()
  void loadEnrollmentKey()
}
const loadEnrollmentKey = async () => {
  Object.assign(enrollKey, { key: '', ...emptyKeyCommands() })
  try {
    const result = await api('api/agents/enrollment-key')
    Object.assign(enrollKey, { configured: !!result?.configured, panel_url: result?.panel_url || '' })
  } catch { /* the one-time address above still works */ }
}
const createEnrollmentKey = async () => {
  if (enrollKey.configured && !window.confirm(i18n.global.t('agent.keyRegenerateConfirm'))) return
  enrollKey.loading = true
  try {
    const result = await api('api/agents/enrollment-key', { method: 'POST', body: '{}' })
    const commands = emptyKeyCommands()
    for (const name of Object.keys(commands) as KeyCommandName[]) commands[name] = String(result[name] || '')
    Object.assign(enrollKey, {
      configured: true,
      panel_url: result.panel_url || enrollKey.panel_url,
      key: result.key || '',
      ...commands,
    })
  } catch (error: any) { push.error({ message: error?.message || i18n.global.t('agent.createFailed') }) }
  finally { enrollKey.loading = false }
}
const revokeEnrollmentKey = async () => {
  if (!window.confirm(i18n.global.t('agent.keyRevokeConfirm'))) return
  enrollKey.loading = true
  try {
    await api('api/agents/enrollment-key/revoke', { method: 'POST', body: '{}' })
    Object.assign(enrollKey, { configured: false, key: '', ...emptyKeyCommands() })
  } catch (error: any) { push.error({ message: error?.message || i18n.global.t('agent.createFailed') }) }
  finally { enrollKey.loading = false }
}
const closeEnrollment = () => { enroll.visible = false; if (enroll.pairURL) void loadNodes() }
const createEnrollmentAPI = async () => {
  enroll.apiLoading = true
  try {
    const result = await api('api/agents/enrollment-link', { method: 'POST', body: '{}' })
    enroll.pairURL = result.simple_address || result.connect_url || ''
    enroll.pairExpiresAt = Number(result.pair_expires_at || 0)
    enroll.command = result.command || ''
    enroll.managedCommand = result.managed_command || ''
    enroll.cnManagedCommand = result.cn_managed_command || ''
  } catch (error: any) { push.error({ message: error?.message || i18n.global.t('agent.createFailed') }) }
  finally { enroll.apiLoading = false }
}
const rotateNode = async (node: AgentNode) => {
  try {
    const result = await api(`api/agents/${node.id}/rotate`, { method: 'POST', body: '{}' })
    Object.assign(enroll, {
      visible: true,
      apiLoading: false,
      pairURL: result.simple_address || result.connect_url || result.pair_url || '',
      pairExpiresAt: Number(result.pair_expires_at || 0),
      command: result.command,
      managedCommand: result.managed_command || '',
      cnManagedCommand: result.cn_managed_command || '',
    })
  } catch (error: any) { push.error({ message: error?.message || i18n.global.t('agent.rotateFailed') }) }
}
const openLocalConnect = () => Object.assign(connect, { visible: true, loading: false, url: '', insecure: false })
const loadControllerMode = async () => {
  controllerModeLoading.value = true
  await dataStore.loadControllerMode(true)
  controllerModeLoading.value = false
}
const loadLocalConnection = async () => {
  try {
    const result = await api('api/agents/local-connection')
    Object.assign(localConnection, { supported: false, installed: false, configured: false, running: false, panel_url: '', public_url: '', insecure: false }, result || {})
  } catch { /* unsupported platforms remain read-only */ }
}
const enableControllerMode = async () => {
  controllerModeLoading.value = true
  try {
    const result = await api('api/controller-mode', { method: 'POST', body: JSON.stringify({ profile: 'full' }) })
    dataStore.assignControllerMode(result)
    push.success({ message: i18n.global.t('setting.roleUpdated') })
  } catch (error: any) {
    push.error({ message: error?.message || serverRequirementText.value })
  } finally { controllerModeLoading.value = false }
}
const connectToController = async () => {
  if (connect.loading) return
  if (localConnection.configured && !window.confirm(i18n.global.t('agent.reconnectConfirm'))) return
  connect.loading = true
  try {
    const result = await api('api/agents/connect-local', {
      method: 'POST',
      body: JSON.stringify({ connect_url: connect.url.trim(), public_url: resolveFrontendUrl(), insecure: connect.insecure }),
    })
    connect.visible = false
    push.success({ message: i18n.global.t('agent.connectSuccess', { panel: result?.panel_url || '-' }) })
    await loadLocalConnection()
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('agent.connectFailed') })
  } finally { connect.loading = false }
}
const disconnectController = async () => {
  if (!window.confirm(i18n.global.t('agent.disconnectConfirm'))) return
  connect.loading = true
  try {
    const result = await api('api/agents/disconnect-local', { method: 'POST', body: '{}' })
    Object.assign(localConnection, result || { configured: false, running: false, panel_url: '' })
    connect.visible = false
    push.success({ message: i18n.global.t('agent.disconnectSuccess') })
  } catch (error: any) {
    push.error({ message: error?.message || i18n.global.t('agent.connectFailed') })
  } finally { connect.loading = false }
}
const toDateInput = (unix?: number) => {
  if (!unix) return ''
  const date = new Date(unix * 1000)
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}
const openEdit = (node: AgentNode) => {
  const meta = { ...defaultMeta() }
  for (const key of Object.keys(meta) as (keyof AgentNodeMeta)[]) {
    if (node[key] != null) (meta as any)[key] = node[key]
  }
  if (!meta.billing_cycle) meta.billing_cycle = 30
  Object.assign(edit, {
    visible: true, loading: false, id: node.id, name: node.name, publicHost: node.public_host || '', meta,
    expireDate: toDateInput(node.expire_at), trafficLimitGiB: node.traffic_limit ? +(node.traffic_limit / 1024 ** 3).toFixed(2) : 0,
  })
}
const saveNode = async () => {
  edit.loading = true
  try {
    const meta: AgentNodeMeta = {
      ...edit.meta,
      price: Number(edit.meta.price) || 0,
      sort_weight: Math.trunc(Number(edit.meta.sort_weight) || 0),
      traffic_reset_day: Math.trunc(Number(edit.meta.traffic_reset_day) || 1),
      // Expiry is the end of the chosen local day.
      expire_at: edit.expireDate ? Math.floor(new Date(`${edit.expireDate}T23:59:59`).getTime() / 1000) : 0,
      traffic_limit: Math.max(0, Math.round((Number(edit.trafficLimitGiB) || 0) * 1024 ** 3)),
    }
    await api(`api/agents/${edit.id}`, { method: 'PATCH', body: JSON.stringify({ name: edit.name.trim(), public_host: edit.publicHost.trim(), meta }) })
    edit.visible = false
    push.success({ message: i18n.global.t('agent.updateSuccess') })
    await loadNodes()
  } catch (error: any) { push.error({ message: error?.message || i18n.global.t('agent.updateFailed') }) }
  finally { edit.loading = false }
}
const askDelete = (node: AgentNode) => Object.assign(removeDialog, { visible: true, loading: false, node })
const deleteNode = async () => {
  if (!removeDialog.node) return
  removeDialog.loading = true
  try {
    await api(`api/agents/${removeDialog.node.id}/delete`, { method: 'POST', body: '{}' })
    removeDialog.visible = false
    await loadNodes()
  } catch (error: any) { push.error({ message: error?.message || i18n.global.t('agent.deleteFailed') }) }
  finally { removeDialog.loading = false }
}

const openDetail = (node: AgentNode) => void router.push(`/agents/${node.id}`)
const manageInbounds = (node: AgentNode) => { if (canControl.value && node.managed) void router.push(`/agents/${node.id}/inbounds`) }
const toggleSelect = (id: number, enabled: boolean | null) => {
  if (enabled && !selected.value.includes(id)) selected.value.push(id)
  if (!enabled) selected.value = selected.value.filter(value => value !== id)
}
const toggleExpand = (id: number) => {
  expanded.value = expanded.value.includes(id) ? expanded.value.filter(value => value !== id) : [...expanded.value, id]
}
const selectAllControllable = () => { selected.value = nodes.value.filter(node => node.controllable).map(node => node.id) }
const batchCmd = async (type: string, args?: Record<string, any>) => {
  if (!canControl.value || !selected.value.length) return
  batch.loading = true
  try {
    batch.results = await api('api/agents/batch-command', { method: 'POST', body: JSON.stringify({ ids: selected.value, type, args: args || {} }) }) || []
    batch.resultVisible = true
    push.success({ message: `${batch.results.filter((item: any) => item.ok).length}/${batch.results.length} OK` })
    await loadNodes()
  } catch (error: any) { push.error({ message: error?.message || i18n.global.t('agent.controlFailed') }) }
  finally { batch.loading = false }
}

const latencyLabel = (node: AgentNode) => node.online && node.latency?.last_ms != null ? `${node.latency.last_ms} ms` : '-'
const latencyClass = (node: AgentNode) => {
  if (!node.online || node.latency?.last_ms == null) return 'text-medium-emphasis'
  const loss = Number(node.latency.loss_pct || 0)
  return loss >= 20 || node.latency.last_ms >= 250 ? 'text-error' : loss > 0 || node.latency.last_ms >= 100 ? 'text-warning' : 'text-success'
}
const loadText = (node: AgentNode) => {
  const load = node.report.load?.load1
  const parts = [load == null ? '-' : load.toFixed(2)]
  if (node.report.tcp_conns != null || node.report.udp_conns != null) parts.push(`TCP ${node.report.tcp_conns ?? 0} · UDP ${node.report.udp_conns ?? 0}`)
  return parts.join(' · ')
}
const nodeFacts = (node: AgentNode) => [
  { label: 'CPU', value: node.report.cpu_model ? `${node.report.cpu_model} (x${node.report.cpu_cores || '?'})` : `${node.report.cpu_cores || '-'} ${t('agent.cpuCores')}` },
  { label: t('monitor.arch'), value: node.report.arch || '-' },
  { label: t('monitor.virtualization'), value: node.report.virtualization || '-' },
  { label: t('monitor.os'), value: [osName(node), node.report.kernel].filter(Boolean).join(' · ') || '-' },
  { label: t('monitor.ram'), value: usageText(node.report.memory) },
  { label: t('monitor.swap'), value: usageText(node.report.swap) },
  { label: t('agent.disk'), value: usageText(node.report.disk) },
  { label: t('monitor.loadConns'), value: loadText(node) },
  { label: t('agent.addresses'), value: [...(node.report.ipv4 || []), ...(node.report.ipv6 || [])].join(', ') || node.remote_ip || '-' },
  { label: t('agent.lastSeen'), value: node.last_seen ? new Date(node.last_seen * 1000).toLocaleString() : '-' },
]
const copy = async (value: string) => {
  try { await copyText(value); push.success({ message: i18n.global.t('success') }) }
  catch { push.error({ message: i18n.global.t('failed') }) }
}

onMounted(() => {
  void Promise.all([loadControllerMode(), loadLocalConnection(), loadNodes()])
  document.addEventListener('visibilitychange', handleVisibilityChange)
  document.addEventListener('keydown', handleKey)
  refreshTimer = window.setInterval(() => { if (!document.hidden) void loadNodes() }, 5000)
  clockTimer = window.setInterval(() => { now.value = new Date() }, 1000)
})
onBeforeUnmount(() => {
  if (refreshTimer) window.clearInterval(refreshTimer)
  if (clockTimer) window.clearInterval(clockTimer)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  document.removeEventListener('keydown', handleKey)
})
</script>

<style scoped>
.monitor-page { display: grid; gap: 14px; }
.monitor-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.monitor-heading h1 { margin: 0; font-size: 1.35rem; line-height: 1.4; letter-spacing: 0; }
.monitor-heading p { margin: 4px 0 0; color: rgba(var(--v-theme-on-surface), 0.62); font-size: 0.9rem; }
.heading-actions, .batch-bar, .list-summary { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }

.stat-card { position: relative; padding: 14px 16px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 12px; background: rgb(var(--v-theme-surface)); }
.stat-settings { position: absolute; top: 6px; right: 6px; }
.stat-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 8px 12px; padding-right: 22px; }
.stat-item { min-width: 0; display: flex; flex-direction: column; }
.stat-item span { color: rgba(var(--v-theme-on-surface), 0.6); font-size: 0.82rem; }
.stat-item strong { font-weight: 500; font-size: 0.98rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.control-bar { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
.search-field { flex: 1 1 240px; max-width: 448px; }
.control-right { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.sort-field { width: 150px; }
.control-label { color: rgba(var(--v-theme-on-surface), 0.6); font-size: 0.82rem; white-space: nowrap; }
.group-bar { display: flex; align-items: center; gap: 10px; overflow-x: auto; margin-top: -4px; }
.group-toggle { flex-shrink: 0; }
.list-summary { color: rgba(var(--v-theme-on-surface), 0.6); font-size: 0.84rem; margin-top: -4px; }
.batch-bar { padding: 10px 12px; border: 1px solid rgba(var(--v-theme-primary), 0.25); border-radius: 8px; background: rgba(var(--v-theme-primary), 0.06); }
.batch-shell { flex: 1 1 190px; max-width: 260px; }
.empty-state { padding: 40px 16px; text-align: center; color: rgba(var(--v-theme-on-surface), 0.56); display: grid; gap: 4px; }

.node-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 16px; }
.node-card { min-width: 0; display: flex; flex-direction: column; gap: 10px; padding: 14px 16px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 12px; background: rgb(var(--v-theme-surface)); cursor: pointer; transition: all 0.2s; }
.node-card:hover { box-shadow: 0 10px 24px rgba(0, 0, 0, 0.1); background: rgba(var(--v-theme-primary), 0.035); }
.node-card--offline { opacity: 0.72; }
.node-card__head { display: flex; align-items: flex-start; justify-content: space-between; gap: 8px; }
.node-card__identity { min-width: 0; display: flex; align-items: flex-start; gap: 8px; }
.node-card__title { min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.node-card__title strong { font-size: 1.05rem; font-weight: 700; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.node-card__title small { color: rgba(var(--v-theme-on-surface), 0.6); font-size: 0.74rem; }
.node-card__status { display: flex; align-items: center; gap: 2px; flex-shrink: 0; }
.node-flag { font-size: 1.3rem; line-height: 1.4; }
.status-badge { padding: 1px 8px; border-radius: 4px; font-size: 0.74rem; font-weight: 500; white-space: nowrap; }
.status-badge--on { color: #218358; background: rgba(48, 164, 108, 0.16); }
.status-badge--off { color: #ce2c31; background: rgba(229, 72, 77, 0.16); }
.node-card__body { display: flex; flex-direction: column; gap: 8px; }
.usage-stack { display: flex; flex-direction: column; gap: 6px; }
.info-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; font-size: 0.82rem; min-width: 0; }
.info-row > span { color: rgba(var(--v-theme-on-surface), 0.6); white-space: nowrap; }
.info-row > strong { font-weight: 500; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: flex; align-items: center; }
.info-row--sub { margin-top: -4px; font-size: 0.72rem; }
.info-row--sub > span { color: rgba(var(--v-theme-on-surface), 0.6); }
.core-badges { gap: 6px; }
.core-badge { padding: 0 5px; border-radius: 4px; font-size: 0.68rem; color: rgba(var(--v-theme-on-surface), 0.45); background: rgba(var(--v-theme-on-surface), 0.07); }
.core-badge--on { color: #218358; background: rgba(48, 164, 108, 0.16); }
.mobile-only { display: none !important; }

.node-table-wrap { overflow-x: auto; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 12px; background: rgb(var(--v-theme-surface)); }
.node-table { width: 100%; min-width: 1180px; border-collapse: collapse; font-size: 0.82rem; }
.node-table th { padding: 10px 8px; text-align: left; font-weight: 500; color: rgba(var(--v-theme-on-surface), 0.62); white-space: nowrap; cursor: pointer; user-select: none; border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); }
.node-table td { padding: 9px 8px; white-space: nowrap; border-bottom: 1px solid rgba(var(--v-border-color), calc(var(--v-border-opacity) * 0.7)); }
.node-row { cursor: pointer; transition: background 0.15s; }
.node-row:hover, .node-row--open { background: rgba(var(--v-theme-primary), 0.05); }
.node-row--offline { opacity: 0.72; }
.col-expand, .col-menu { width: 34px; cursor: default !important; }
.col-bar { width: 100px; min-width: 100px; }
.col-tags { max-width: 220px; white-space: normal !important; }
.table-name { display: flex; align-items: center; gap: 8px; }
.table-name strong, .table-name small { display: block; }
.table-name small { color: rgba(var(--v-theme-on-surface), 0.58); font-size: 0.7rem; }
.node-expand td { background: rgba(var(--v-theme-primary), 0.03); padding: 14px 16px; white-space: normal; }
.expand-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 10px 18px; }
.expand-grid span, .expand-grid strong { display: block; }
.expand-grid span { font-weight: 600; font-size: 0.76rem; }
.expand-grid strong { font-weight: 400; font-size: 0.8rem; color: rgba(var(--v-theme-on-surface), 0.72); word-break: break-word; }
.expand-actions { margin-top: 12px; }

.edit-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px 12px; }
.edit-grid .span-2 { grid-column: 1 / -1; }
.edit-pair { display: flex; gap: 8px; align-items: flex-start; }
.currency-field { flex: 0 0 90px; }
.reset-field { flex: 0 0 110px; }
.flag-preview { min-width: 1.2em; font-size: 1.1rem; }

@media (max-width: 800px) {
  .monitor-heading { flex-wrap: wrap; justify-content: center; }
  .monitor-heading > div:first-child { text-align: center; flex: 1; }
  .heading-actions { justify-content: center; width: 100%; }
}
@media (max-width: 600px) {
  .stat-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .stat-item strong { font-size: 0.84rem; }
  .search-field { max-width: none; flex-basis: 100%; }
  .control-bar, .control-right { justify-content: space-between; width: 100%; }
  .sort-field { flex: 1 1 120px; width: auto; }
  .node-grid { gap: 8px; }
  .desktop-only { display: none !important; }
  .mobile-only { display: flex !important; }
  .node-card__title small.mobile-only { display: block !important; }
  .usage-stack { flex-direction: row; gap: 14px; }
  .usage-stack > * { flex: 1 1 0; }
  .usage-stack :deep(.usage-bar__detail) { display: none; }
  .edit-grid { grid-template-columns: minmax(0, 1fr); }
}
</style>
