<template>
  <q-page class="items-center justify-evenly">
    <p class="text-h6">{{ iface?.metadata.name }}</p>
    <p>
      <q-banner dense class="text-white bg-red" v-if="iface?.status.error">
        Error: {{ iface?.status.error }}
      </q-banner>
    </p>
    <p>Status: {{ iface?.status.link?.up ? 'up' : 'down' }}</p>
    <p>MAC address: {{ iface?.status.link?.mac }}</p>
    <p>IP address: {{ iface?.status.link?.ip4 }}</p>
    <wifi-settings
      :interface-name="iface?.metadata.name"
      v-if="iface?.metadata.name && isWifiInterface(iface)"
    />
  </q-page>
</template>

<script lang="ts">
import { computed, defineComponent, toRefs } from 'vue';
import WifiSettings from 'src/components/WifiSettings.vue';
import { useRoute } from 'vue-router';
import {
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1alpha1_NetworkInterface as NetworkInterface,
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1alpha1_NetworkLinkStatus as NetworkLinkStatus,
} from 'src/gen';
import { useNetworkInterfaceStore } from 'src/stores/resources';

export default defineComponent({
  name: 'NetworkInterfacePage',
  components: { WifiSettings },
  setup() {
    const store = useNetworkInterfaceStore();
    store.sync();
    const state = {
      iface: computed(() =>
        store.resources.find(
          (r) => r.metadata.name == (useRoute().params.name as string)
        )
      ),
      isWifiInterface: (iface?: NetworkInterface) =>
        iface?.status.link?.type == NetworkLinkStatus.type.WIFI,
    };
    return {
      ...toRefs(state),
    };
  },
});
</script>
