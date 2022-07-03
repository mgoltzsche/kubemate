<template>
  <div v-if="!device && synchronizing">Loading...</div>
  <div v-if="!device && !synchronizing">Device not found</div>
  <div v-if="device">
    <q-card flat class="my-card">
      <q-card-section>
        <div class="text-h6">{{ device.metadata.name }}</div>
      </q-card-section>

      <q-card-section class="q-pt-none q-gutter-y-md">
        <div>
          Address:
          <a
            :href="`${device.status.address}#/devices/${device.metadata.name}`"
            >{{ device.status.address }}</a
          >
        </div>
        <div>Status: {{ device.status.state }} {{ device.spec.mode }}</div>
      </q-card-section>

      <q-card-section class="q-pt-none" v-if="device.status.message">
        {{ device.status.message }}
      </q-card-section>

      <q-card-section v-if="currentDeviceName != device.metadata.name">
        <q-btn
          color="secondary"
          label="Configure device"
          @click="switchDevice"
        />
      </q-card-section>

      <q-card-section
        class="q-pt-none q-gutter-y-md"
        style="max-width: 350px"
        v-if="currentDeviceName == device.metadata.name"
      >
        <q-separator inset />
        <div>
          <q-btn-toggle
            v-model="device.spec.mode"
            :options="availableDeviceModes"
          />

          <q-tab-panels
            v-model="device.spec.mode"
            animated
            class="shadow-2 rounded-borders"
          >
            <q-tab-panel name="server">
              The device should act as a server.
            </q-tab-panel>
            <q-tab-panel name="agent">
              <div>The device should join a server:</div>
              <q-card-section>
                <q-select
                  filled
                  clearable
                  bottom-slots
                  v-model="selectedServer"
                  :options="availableDevices"
                  :label="
                    availableDevices.length == 0 ? 'No server found' : 'server'
                  "
                  :color="availableDevices.length == 0 ? 'negative' : 'gray'"
                >
                  <template v-slot:hint
                    >The selected server manages all data and controls this
                    device.</template
                  >
                </q-select>
              </q-card-section>
              <q-card-actions>
                <q-btn
                  clickable
                  :disable="!selectedServer"
                  label="Delete join token"
                  color="secondary"
                  @click="deleteJoinToken"
                />
              </q-card-actions>
            </q-tab-panel>
          </q-tab-panels>
        </div>
        <q-btn color="primary" label="Apply" @click="apply" />
      </q-card-section>
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
import { ApiError } from 'src/k8sclient/request';

function serverJoinTokenRequestURL(server: Device) {
  const addrRegex = new RegExp('https://([^/]+)');
  const m = window.location.href.match(addrRegex);
  const addr = m ? m[1] : '';
  return `${server.status.address}/#/setup/request-join-token/${addr}`;
}

const kc = new apiclient.KubeConfig();
const client = kc.newClient<DeviceToken>(
  '/apis/kubemate.mgoltzsche.github.com/v1/devicetokens'
);

export default defineComponent({
  name: 'DeviceDetails',
  props: {
    deviceName: {
      type: String,
      required: true,
    },
  },
  setup(props) {
    const store = useDeviceStore();
    store.sync();
    const selectedServer = ref(null as unknown) as Ref<{ value: Device }>;
    const quasar = useQuasar();
    const state = reactive({
      selectedServer: selectedServer,
      synchronizing: store.synchronizing,
      currentDeviceName: computed(
        () => store.resources.find((d) => d.status.current)?.metadata.name || ''
      ),
      device: computed(() =>
        store.resources.find((d) => d.metadata.name == props.deviceName)
      ),
      availableDevices: computed(() =>
        store.resources
          .filter((d) => d.metadata.name != props.deviceName)
          .map((d) => ({ label: d.metadata.name, value: d }))
      ),
      availableDeviceModes: [
        { label: 'Server', value: DeviceSpec.mode.SERVER },
        { label: 'Agent', value: DeviceSpec.mode.AGENT },
      ],
      deleteJoinToken: async () => {
        if (selectedServer.value && selectedServer.value.value.metadata.name) {
          try {
            await client.delete(selectedServer.value.value.metadata.name);
            console.log('deleted join token');
          } catch (e: any) {
            quasar.notify({
              type: 'negative',
              message: e.body?.message
                ? `${e.message}: ${e.body?.message}`
                : e.message,
            });
          }
        }
      },
      apply: () => {
        const d = store.resources.find(
          (d) => d.metadata.name == props.deviceName
        );
        if (!d) {
          // TODO: error notification
          return;
        }
        if (selectedServer.value == null) {
          // TODO: error notification
          return;
        }
        const serverName = selectedServer.value.value.metadata.name;
        const serverAddr = selectedServer.value.value.status.address;
        if (!serverName || !serverAddr) {
          // TODO: error notification
          return;
        }
        d.spec.mode = DeviceSpec.mode.AGENT;
        d.spec.server = serverName;
        console.log('update device', d);
        store.client
          .update(d)
          .then(() => {
            console.log('device updated');
            client
              .get(serverName)
              .then(() => {
                console.log(
                  `join token for server device ${serverName} already exists on agent device ${props.deviceName}`
                );
              })
              .catch((e) => {
                const url = serverJoinTokenRequestURL(
                  selectedServer.value.value
                );
                console.log(
                  `join token for server ${serverName} does not exist - redirecting user to retrieve token to ${url}`
                );
                window.location.href = url;
              });
          })
          .catch((e) => {
            quasar.notify({
              type: 'negative',
              message: e.body?.message
                ? `${e.message}: ${e.body?.message}`
                : e.message,
            });
          });
      },
      switchDevice: () => {
        const a = store.resources.find(
          (d) => d.metadata.name == props.deviceName
        )?.status.address;
        if (a) window.location.href = `${a}/#/devices/${props.deviceName}`;
      },
    });
    return { ...toRefs(state) };
  },
});
</script>
