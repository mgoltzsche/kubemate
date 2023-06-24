<template>
  <q-dialog v-model="open" ref="dialog" @hide="closeDialog">
    <q-card style="min-width: 350px">
      <q-card-section>
        <div class="text-h6">Device address</div>
      </q-card-section>

      <q-card-section class="q-pt-none">
        <q-input
          dense
          v-model="deviceAddress"
          autofocus
          placeholder="https://mymachine"
          hint="Device name or URL"
          @keyup.enter="addDevice"
        />
      </q-card-section>

      <q-card-actions align="right" class="text-primary">
        <q-btn flat label="Cancel" v-close-popup />
        <q-btn
          flat
          label="Add device"
          @click="addDevice"
          :disable="deviceAddress == ''"
        />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script lang="ts">
import { defineComponent, ref } from 'vue';
import { useDeviceDiscoveryStore } from 'src/stores/resources';
import { com_github_mgoltzsche_kubemate_pkg_apis_devices_v1alpha1_DeviceDiscoverySpec as DeviceDiscoverySpec } from 'src/gen';
import { catchError } from 'src/notify';

const deviceAddress = ref('');

export default defineComponent({
  name: 'DeviceAddressDialog',
  props: {
    value: {
      type: Boolean,
      default: false,
    },
    title: {
      type: String,
    },
  },
  data() {
    return {
      open: this.value,
    };
  },
  methods: {
    closeDialog() {
      this.$emit('input', false);
    },
    addDevice() {
      var addr = deviceAddress.value;
      if (!addr) return;
      if (!addr.startsWith('https://')) {
        addr = `https://${addr}`;
      }
      const k8sName = `host-${deviceAddress.value
        .toLowerCase()
        .replace(/^https?:\/\/([a-z0-9\._-]+)/, '$1')}`
        .replace(/[^a-z0-9]+/, '-')
        .replace(/[^a-z0-9]$/, '');
      const d = {
        metadata: {
          generateName: `${k8sName}-`,
        },
        spec: {
          address: addr,
          mode: DeviceDiscoverySpec.mode.SERVER,
        },
      };
      const store = useDeviceDiscoveryStore();
      store.sync();
      catchError(
        store.client.create(d).then(() => {
          deviceAddress.value = '';
          (this.$refs.dialog as any).hide();
        })
      );
    },
  },
  setup() {
    return {
      deviceAddress,
    };
  },
});
</script>
