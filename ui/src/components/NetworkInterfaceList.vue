<template>
  <q-list>
    <q-item
      v-for="iface in networkInterfaces"
      :key="iface.metadata?.name"
      clickable
      v-ripple
      :to="`/networkinterfaces/${iface.metadata?.name}`"
    >
      <q-item-section avatar>
        <q-avatar :text-color="statusColor(iface)">
          <q-icon :name="icon(iface)" />
        </q-avatar>
      </q-item-section>
      <q-item-section>
        <q-item-label lines="1">{{ iface.metadata?.name }}</q-item-label>
        <q-item-label caption lines="1"
          >{{ iface.status?.link?.type }},
          {{ iface.status.link?.up ? 'up' : 'down' }}</q-item-label
        >
      </q-item-section>
      <q-item-section side v-if="iface.status?.error">
        <q-avatar text-color="negative">
          <q-icon name="warning" />
        </q-avatar>
      </q-item-section>
    </q-item>
  </q-list>
</template>

<script lang="ts">
import { defineComponent } from 'vue';
import { storeToRefs } from 'pinia';
import { useNetworkInterfaceStore } from 'src/stores/resources';
import {
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_NetworkInterface as NetworkInterface,
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_NetworkLinkStatus as NetworkLinkStatus,
} from 'src/gen';

export default defineComponent({
  name: 'NetworkInterfaceList',
  setup() {
    const store = useNetworkInterfaceStore();
    store.sync();
    const { resources } = storeToRefs(store);
    return {
      networkInterfaces: resources,
      statusColor: (iface: NetworkInterface) =>
        iface.status.link?.up ? 'positive' : 'gray',
      icon: (iface: NetworkInterface) => {
        switch (iface.status.link?.type) {
          case NetworkLinkStatus.type.WIFI:
            return 'wifi';
          case NetworkLinkStatus.type.ETHER:
            return 'cable';
          default:
            return 'settings_ethernet';
        }
      },
    };
  },
});
</script>
