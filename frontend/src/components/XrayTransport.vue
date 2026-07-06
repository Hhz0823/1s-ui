<template>
  <v-card :subtitle="$t('objects.transport')">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-select
          hide-details
          label="Xray Transport"
          :items="transportTypes"
          v-model="transport.type"
        ></v-select>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="transport.type == 'xhttp'">
        <v-select
          hide-details
          label="XHTTP Mode"
          :items="xhttpModes"
          v-model="transport.mode"
        ></v-select>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="hasHost">
        <v-text-field hide-details label="Host" v-model="transport.host"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="hasPath">
        <v-text-field hide-details :label="$t('transport.path')" v-model="transport.path"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="transport.type == 'grpc'">
        <v-text-field hide-details label="Service Name" v-model="transport.service_name"></v-text-field>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">
export default {
  props: ['data'],
  data() {
    return {
      transportTypes: [
        { title: 'XHTTP', value: 'xhttp' },
        { title: 'TCP', value: 'tcp' },
        { title: 'WebSocket', value: 'ws' },
        { title: 'gRPC', value: 'grpc' },
        { title: 'HTTPUpgrade', value: 'httpupgrade' },
      ],
      xhttpModes: ['auto', 'packet-up', 'stream-up', 'stream-one'],
    }
  },
  computed: {
    transport() {
      if (!this.$props.data.transport || Object.keys(this.$props.data.transport).length == 0) {
        this.$props.data.transport = { type: 'xhttp', path: '/xhttp', mode: 'auto' }
      }
      return this.$props.data.transport
    },
    hasHost(): boolean {
      return ['xhttp', 'ws', 'httpupgrade'].includes(this.transport.type)
    },
    hasPath(): boolean {
      return ['xhttp', 'ws', 'httpupgrade'].includes(this.transport.type)
    },
  },
  watch: {
    'transport.type'(value: string) {
      if (value == 'xhttp') {
        this.transport.path = this.transport.path || '/xhttp'
        this.transport.mode = this.transport.mode || 'auto'
      } else if (value == 'grpc') {
        this.transport.service_name = this.transport.service_name || ''
      } else if (['ws', 'httpupgrade'].includes(value)) {
        this.transport.path = this.transport.path || '/'
      }
    },
  },
}
</script>
