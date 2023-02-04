<template>
  <div v-if="!iface && synchronizing">Loading...</div>
  <div v-if="!iface && !synchronizing">Device not found</div>
  <div v-if="iface">
    <q-card flat>
      <q-card-section>
        <div class="text-h6">Wifi settings</div>
      </q-card-section>

      <q-card-section class="q-pt-none q-gutter-y-md">
        <div>
          <div style="max-width: 300px">
            <q-input
              filled
              v-model="wifi.countryCode"
              label="Country code"
              mask="AA"
              fill-mask
              size="2"
            />
          </div>
          <div class="q-gutter-sm">
            <q-radio
              v-model="wifi.mode"
              :val="mode.value"
              :label="mode.label"
              v-for="mode in availableWifiModes"
              v-bind:key="mode.value"
            />
          </div>
          <q-tab-panels
            v-model="wifi.mode"
            animated
            class="shadow-2 rounded-borders"
          >
            <q-tab-panel name="accesspoint">
              <q-btn
                color="secondary"
                label="Set password"
                @click="
                  promptPassword(
                    'accesspoint',
                    iface?.spec.wifi?.accessPoint.SSID || ''
                  )
                "
              />
            </q-tab-panel>
            <q-tab-panel name="station">
              <q-card-section>
                <p>Connect with wifi network:</p>
                <div v-if="scanning">scanning...</div>
                <div v-if="!scanning && availableNetworks.length == 0">
                  No wifi networks found!
                  <div v-if="iface?.spec.wifi?.mode !== stationMode">
                    <i
                      >You may need to activate station mode in order to be able
                      to scan wifi networks!
                    </i>
                  </div>
                </div>
                <q-virtual-scroll
                  style="max-height: 300px"
                  :items="availableNetworks"
                  separator
                  v-slot="{ item }"
                  v-if="availableNetworks.length > 0"
                >
                  <q-item tag="label" v-ripple :key="item.metadata.name">
                    <q-item-section avatar>
                      <q-radio
                        v-model="wifi.station.SSID"
                        :val="item.data.ssid"
                        v-on:click="
                          promptPassword(item.metadata.name, item.data.ssid)
                        "
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
    <wifi-password-dialog
      v-model="showWifiConnectPassword"
      :name="password.resourceName"
      :ssid="password.ssid"
      :key="password.resourceName"
    />
  </div>
</template>

<script lang="ts">
import { computed, defineComponent, reactive, Ref, toRefs, ref } from 'vue';
//import { useDeviceStore } from 'src/stores/resources';
import apiclient from 'src/k8sclient';
import { catchError } from 'src/notify';
import {
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_WifiSpec as WifiSpec,
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_WifiNetwork as WifiNetwork,
} from 'src/gen';
import sync from 'src/stores/sync';
import { CancelablePromise } from 'src/k8sclient/CancelablePromise';
import { useNetworkInterfaceStore } from 'src/stores/resources';
import WifiPasswordDialog from 'src/components/WifiPasswordDialog.vue';

interface WifiConnectPassword {
  resourceName: string;
  ssid: string;
}

const kc = new apiclient.KubeConfig();
const wifiNetworkClient = kc.newClient<WifiNetwork>(
  '/apis/kubemate.mgoltzsche.github.com/v1',
  'wifinetworks'
);

let wifiNetworkSync: CancelablePromise<void> | null = null;
let netIfaceSync: CancelablePromise<void> | null = null;

export default defineComponent({
  name: 'WifiSettings',
  components: { WifiPasswordDialog },
  props: {
    interfaceName: {
      type: String,
      required: true,
    },
  },
  beforeUnmount() {
    wifiNetworkSync?.cancel();
    netIfaceSync?.cancel();
  },
  setup(props) {
    const ifaceStore = useNetworkInterfaceStore();
    //const deviceStore = useDeviceStore();
    const wifi = ref<WifiSpec>({
      mode: WifiSpec.mode.DISABLED,
      accessPoint: { SSID: '' },
      station: { SSID: '' },
    });
    ifaceStore.sync(() => {
      const r = ifaceStore.resources.find(
        (r) => r.metadata.name == props.interfaceName
      );
      if (!r) return;
      wifi.value = JSON.parse(JSON.stringify(r.spec.wifi));
    });
    /*deviceStore.sync(() => {
      const d = deviceStore.resources.find((d) => d.status.current);
      if (!d) return;
      wifi.value = JSON.parse(JSON.stringify(d.spec.wifi));
    });*/
    //const availableInterfaces = ref([]) as Ref<NetworkInterface[]>;
    //const loading = ref(false);
    //netIfaceSync = sync(networkInterfaceClient, availableInterfaces, loading);
    const availableNetworks = ref([]) as Ref<WifiNetwork[]>;
    const scanning = ref(false);
    wifiNetworkSync = sync(wifiNetworkClient, availableNetworks, scanning);
    const showWifiConnectPassword = ref(false);
    const password = ref<WifiConnectPassword>({
      resourceName: '',
      ssid: '',
    });
    const state = reactive({
      synchronizing: ifaceStore.synchronizing,
      availableNetworks,
      iface: computed(() =>
        ifaceStore.resources.find((r) => r.metadata.name == props.interfaceName)
      ),
      promptPassword: async (name: string, ssid: string) => {
        password.value = {
          resourceName: name,
          ssid: ssid,
        };
        showWifiConnectPassword.value = true;
      },
      apply: () => {
        const r = ifaceStore.resources.find(
          (r) => r.metadata.name == props.interfaceName
        );
        if (!r) return;
        r.spec.wifi = wifi.value;
        catchError(ifaceStore.client.update(r));
      },
    });
    return {
      ...toRefs(state),
      password,
      showWifiConnectPassword,
      wifi,
      scanning,
      stationMode: WifiSpec.mode.STATION,
      availableWifiModes: [
        { label: 'Disabled', value: WifiSpec.mode.DISABLED },
        { label: 'Access Point', value: WifiSpec.mode.ACCESSPOINT },
        { label: 'Station', value: WifiSpec.mode.STATION },
      ],
    };
  },
});
</script>
