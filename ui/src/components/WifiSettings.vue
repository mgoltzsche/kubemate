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
                  <div v-if="device.spec.wifi.mode !== stationMode">
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
import { useDeviceStore } from 'src/stores/resources';
import apiclient from 'src/k8sclient';
import { catchError, error } from 'src/notify';
import {
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_WifiConfig as WifiConfig,
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_WifiNetwork as WifiNetwork,
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_WifiPassword as WifiPassword,
} from 'src/gen';
import sync from 'src/stores/sync';
import { CancelablePromise } from 'src/k8sclient/CancelablePromise';

const kc = new apiclient.KubeConfig();
const wifiNetworkClient = kc.newClient<WifiNetwork>(
  '/apis/kubemate.mgoltzsche.github.com/v1/wifinetworks'
);
const wifiPasswordClient = kc.newClient<WifiPassword>(
  '/apis/kubemate.mgoltzsche.github.com/v1/wifipasswords'
);

interface WifiConnectPassword {
  resourceName: string;
  ssid: string;
  password: string;
}

let wifiNetworkSync: CancelablePromise<void> | null = null;

export default defineComponent({
  name: 'WifiSettings',
  beforeUnmount() {
    wifiNetworkSync?.cancel();
  },
  setup() {
    const deviceStore = useDeviceStore();
    const wifi = ref<WifiConfig>({
      accessPoint: { SSID: '' },
      station: { SSID: '' },
    });
    deviceStore.sync(() => {
      const d = deviceStore.resources.find((d) => d.status.current);
      if (!d) return;
      wifi.value = JSON.parse(JSON.stringify(d.spec.wifi));
    });
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
      synchronizing: deviceStore.synchronizing,
      availableNetworks: availableNetworks,
      device: computed(() =>
        deviceStore.resources.find((d) => d.status.current)
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
        const d = deviceStore.resources.find((d) => d.status.current);
        if (!d) return;
        d.spec.wifi = wifi.value;
        catchError(deviceStore.client.update(d));
      },
    });
    return {
      ...toRefs(state),
      wifi,
      scanning,
      stationMode: WifiConfig.mode.STATION,
      availableWifiModes: [
        { label: 'Disabled', value: WifiConfig.mode.DISABLED },
        { label: 'Access Point', value: WifiConfig.mode.ACCESSPOINT },
        { label: 'Station', value: WifiConfig.mode.STATION },
      ],
    };
  },
});
</script>
