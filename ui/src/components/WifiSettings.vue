<template>
  <div v-if="!device && synchronizing">Loading...</div>
  <div v-if="!device && !synchronizing">Device not found</div>
  <div v-if="device">
    <q-card flat>
      <q-card-section>
        <div class="text-h6">Wifi settings</div>
      </q-card-section>

      <q-card-section class="q-pt-none q-gutter-y-md">
        <div>
          <div class="q-gutter-sm">
            <q-radio
              v-model="device.spec.wifi.mode"
              :val="mode.value"
              :label="mode.label"
              v-for="mode in availableWifiModes"
              v-bind:key="mode.value"
            />
          </div>
          <q-tab-panels
            v-model="device.spec.wifi.mode"
            animated
            class="shadow-2 rounded-borders"
          >
            <q-tab-panel name="station">
              <q-card-section>
                <div :v-if="availableNetworks.length == 0">
                  No wifi networks found!
                </div>
                <q-virtual-scroll
                  style="max-height: 300px"
                  :items="availableNetworks"
                  separator
                  v-slot="{ item }"
                  :v-if="availableNetworks.length > 0"
                >
                  <q-item tag="label" v-ripple :key="item.metadata.name">
                    <q-item-section avatar>
                      <q-radio
                        v-model="device.spec.wifi.station.SSID"
                        :val="item.data.ssid"
                      />
                    </q-item-section>
                    <q-item-section>
                      <q-item-label>{{ item.data.ssid }}</q-item-label>
                    </q-item-section>
                  </q-item>
                </q-virtual-scroll>
              </q-card-section>
            </q-tab-panel>
          </q-tab-panels>
        </div>
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
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_WifiConfig as WifiConfig,
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_WifiNetwork as WifiNetwork,
} from 'src/gen';
import { useQuasar } from 'quasar';

const kc = new apiclient.KubeConfig();
const wifiNetworkClient = kc.newClient<WifiNetwork>(
  '/apis/kubemate.mgoltzsche.github.com/v1/wifinetworks'
);

export default defineComponent({
  name: 'WifiSettings',
  setup() {
    const deviceStore = useDeviceStore();
    deviceStore.sync();
    const quasar = useQuasar();
    const availableNetworks = ref([]) as Ref<WifiNetwork[]>;
    wifiNetworkClient.list().then((l) => {
      availableNetworks.value = l.items;
    });

    const state = reactive({
      synchronizing: deviceStore.synchronizing,
      availableNetworks: availableNetworks,
      device: computed(() =>
        deviceStore.resources.find((d) => d.status.current)
      ),
      apply: async () => {
        const d = deviceStore.resources.find((d) => d.status.current);
        if (!d) return;
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
    return {
      ...toRefs(state),
      availableWifiModes: [
        { label: 'Disabled', value: WifiConfig.mode.DISABLED },
        { label: 'Access Point', value: WifiConfig.mode.ACCESSPOINT },
        { label: 'Station', value: WifiConfig.mode.STATION },
      ],
    };
  },
});
</script>
