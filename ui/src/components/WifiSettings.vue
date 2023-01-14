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
            <q-tab-panel name="station">
              <q-card-section>
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
                        v-on:click="promptPassword(item)"
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
    <q-dialog v-model="promptWifiConnectPassword">
      <q-card style="min-width: 350px">
        <q-card-section>
          <div class="text-h6">
            Wifi password for {{ wifiConnectPassword.ssid }}
          </div>
        </q-card-section>

        <q-card-section class="q-pt-none">
          <q-input
            dense
            autofocus
            v-model="wifiConnectPassword.password"
            hint="Must have 8..63 characters!"
            :type="showWifiConnectPassword ? 'text' : 'password'"
            @keyup.enter="saveWifiConnectPassword()"
          >
            <template v-slot:append>
              <q-icon
                :name="
                  showWifiConnectPassword ? 'visibility_off' : 'visibility'
                "
                class="cursor-pointer"
                @click="showWifiConnectPassword = !showWifiConnectPassword"
              />
            </template>
          </q-input>
        </q-card-section>

        <q-card-actions align="right" class="text-primary">
          <q-btn flat label="Cancel" v-close-popup />
          <q-btn
            flat
            label="OK"
            v-on:click="saveWifiConnectPassword()"
            :disable="
              wifiConnectPassword.password.length < 8 ||
              wifiConnectPassword.password.length > 63
            "
          />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </div>
</template>

<script lang="ts">
import { computed, defineComponent, reactive, Ref, toRefs, ref } from 'vue';
//import { useDeviceStore } from 'src/stores/resources';
import apiclient from 'src/k8sclient';
import { catchError, error } from 'src/notify';
import {
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_WifiSpec as WifiSpec,
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_WifiNetwork as WifiNetwork,
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_WifiPassword as WifiPassword,
} from 'src/gen';
import sync from 'src/stores/sync';
import { CancelablePromise } from 'src/k8sclient/CancelablePromise';
import { useNetworkInterfaceStore } from 'src/stores/resources';

const kc = new apiclient.KubeConfig();
const wifiNetworkClient = kc.newClient<WifiNetwork>(
  '/apis/kubemate.mgoltzsche.github.com/v1',
  'wifinetworks'
);
const wifiPasswordClient = kc.newClient<WifiPassword>(
  '/apis/kubemate.mgoltzsche.github.com/v1',
  'wifipasswords'
);

interface WifiConnectPassword {
  resourceName: string;
  ssid: string;
  password: string;
}

let wifiNetworkSync: CancelablePromise<void> | null = null;
let netIfaceSync: CancelablePromise<void> | null = null;

export default defineComponent({
  name: 'WifiSettings',
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
    const promptWifiConnectPassword = ref(false);
    const wifiConnectPassword = ref({
      resourceName: '',
      ssid: '',
      password: '',
    }) as Ref<WifiConnectPassword>;

    const state = reactive({
      synchronizing: ifaceStore.synchronizing,
      availableNetworks,
      iface: computed(() =>
        ifaceStore.resources.find((r) => r.metadata.name == props.interfaceName)
      ),
      promptWifiConnectPassword: promptWifiConnectPassword,
      showWifiConnectPassword: false,
      wifiConnectPassword: wifiConnectPassword,
      promptPassword: async (n: WifiNetwork) => {
        const ssid = n.data.ssid;
        try {
          promptWifiConnectPassword.value = true;
          const name = n.metadata.name!;
          try {
            const pw = await wifiPasswordClient.get(name);
            wifiConnectPassword.value = {
              resourceName: name,
              ssid: ssid,
              password: pw.data.password,
            };
          } catch (_) {
            wifiConnectPassword.value = {
              resourceName: name,
              ssid: ssid,
              password: '',
            };
          }
        } catch (e) {
          error(e);
        }
      },
      saveWifiConnectPassword: async () => {
        try {
          const pw = await wifiPasswordClient.get(
            wifiConnectPassword.value.resourceName
          );
          if (wifiConnectPassword.value.password != pw.data.password) {
            pw.data.password = wifiConnectPassword.value.password;
            catchError(
              wifiPasswordClient.update(pw).then(() => {
                promptWifiConnectPassword.value = false;
              })
            );
          }
        } catch (_) {
          catchError(
            wifiPasswordClient
              .create({
                metadata: {
                  name: wifiConnectPassword.value.resourceName,
                },
                data: {
                  password: wifiConnectPassword.value.password,
                },
              })
              .then(() => {
                promptWifiConnectPassword.value = false;
              })
          );
        }
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
