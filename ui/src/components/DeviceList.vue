<template>
  <q-list>
    <q-item
      v-for="device in devices"
      :key="device.metadata.name"
      clickable
      v-ripple
      :to="deviceLinkTo(device)"
      :href="deviceLinkHref(device)"
    >
      <q-item-section avatar>
        <q-avatar color="info" text-color="white"> </q-avatar>
      </q-item-section>
      <q-item-section>
        <q-item-label lines="1">{{ device.metadata.name }}</q-item-label>
        <q-item-label caption lines="1">{{ device.spec.mode }}</q-item-label>
      </q-item-section>
    </q-item>
  </q-list>
</template>

<script lang="ts">
import { defineComponent } from 'vue';
import { useDeviceDiscoveryStore } from 'src/stores/resources';
import { storeToRefs } from 'pinia';
import { com_github_mgoltzsche_kubemate_pkg_apis_devices_v1alpha1_DeviceDiscovery as DeviceDiscovery } from 'src/gen';

function deviceLinkTo(d: DeviceDiscovery) {
  return d.spec.current ? `/devices/${d.metadata.name}` : undefined;
}

function deviceLinkHref(d: DeviceDiscovery) {
  return d.spec.current
    ? undefined
    : `${d.spec.address}/#/devices/${d.metadata.name}`;
}

export default defineComponent({
  name: 'DeviceDiscoveryList',
  setup() {
    const store = useDeviceDiscoveryStore();
    store.sync();
    const { resources } = storeToRefs(store);
    return {
      devices: resources,
      deviceLinkTo: deviceLinkTo,
      deviceLinkHref: deviceLinkHref,
    };
  },
});
</script>
