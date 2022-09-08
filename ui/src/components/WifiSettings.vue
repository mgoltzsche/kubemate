<template>
  <div v-if="!device && synchronizing">Loading...</div>
  <div v-if="!device && !synchronizing">Device not found</div>
  <div v-if="device">
    <q-card flat>
      <q-card-section>
        <div class="text-h6">Wifi settings</div>
      </q-card-section>

      <q-card-section class="q-pt-none q-gutter-y-md">
        <q-checkbox v-model="device.spec.wifi.enabled" label="Hotspot" />
      </q-card-section>
      <q-card-actions>
        <q-btn color="primary" label="Apply" @click="apply" />
      </q-card-actions>
    </q-card>
  </div>
</template>

<script lang="ts">
import { computed, defineComponent, reactive, Ref, toRefs, ref } from 'vue';
import { useDeviceStore } from 'src/stores/resources';
import apiclient from 'src/k8sclient';
import {
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_Device as Device,
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_DeviceSpec as DeviceSpec,
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_DeviceToken as DeviceToken,
} from 'src/gen';
import { useQuasar } from 'quasar';

const kc = new apiclient.KubeConfig();
const client = kc.newClient<DeviceToken>(
  '/apis/kubemate.mgoltzsche.github.com/v1/devicetokens'
);

export default defineComponent({
  name: 'WifiSettings',
  setup() {
    const deviceStore = useDeviceStore();
    deviceStore.sync();
    const quasar = useQuasar();
    const enabled = ref(true); // TODO: init from store

    const state = reactive({
      enabled: enabled,
      synchronizing: deviceStore.synchronizing,
      device: computed(() =>
        deviceStore.resources.find((d) => d.status.current)
      ),
      apply: async () => {
        const d = deviceStore.resources.find((d) => d.status.current);
        if (!d) return;
        //d.spec.wifi.enabled = enabled.value;
        console.log(`setting wifi=${enabled.value}`);
        try {
          await deviceStore.client.update(d);
        } catch (e: any) {
          quasar.notify({
            type: 'negative',
            message: e.body?.message
              ? `${e.message}: ${e.body?.message}`
              : e.message,
          });
        }
      },
    });
    return { ...toRefs(state) };
  },
});
</script>
