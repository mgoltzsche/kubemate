<template>
  <div v-if="!device && synchronizing"><q-skeleton type="rect" /></div>
  <div v-if="!device && !synchronizing">Device not found</div>
  <div v-if="device">
    <q-card flat class="my-card">
      <q-card-section>
        <div class="text-h6">{{ device.metadata.name }}</div>
      </q-card-section>

      <q-card-section class="q-pt-none q-gutter-y-md">
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
          <div class="q-gutter-sm">
            <q-radio
              v-model="deviceSpec.mode"
              :val="mode.value"
              :label="mode.label"
              v-for="mode in availableDeviceModes"
              v-bind:key="mode.value"
            />
          </div>
          <q-tab-panels
            v-model="deviceSpec.mode"
            animated
            class="shadow-2 rounded-borders"
          >
            <q-tab-panel name="server">
              The device should control a cluster.
            </q-tab-panel>
            <q-tab-panel name="agent">
              <div>The device should join a cluster:</div>
              <q-card-section>
                <device-select v-model="deviceSpec.serverAddress" />
              </q-card-section>
              <q-card-actions>
                <q-btn
                  clickable
                  :disable="!deviceSpec.serverAddress"
                  label="Unpair device"
                  title="Delete the cluster join token"
                  color="secondary"
                  @click="deleteJoinToken"
                />
              </q-card-actions>
            </q-tab-panel>
          </q-tab-panels>
        </div>
      </q-card-section>
      <q-card-actions>
        <q-btn color="primary" label="Apply" @click="apply" />
        <q-btn
          color="negative"
          label="Shutdown"
          icon="power_settings_new"
          @click="requestShutdown"
        />
      </q-card-actions>
    </q-card>

    <q-dialog v-model="confirmShutdown" persistent>
      <q-card>
        <q-card-section class="row items-center">
          <q-avatar
            icon="power_settings_new"
            color="primary"
            text-color="white"
          />
          <span class="q-ml-sm"
            >Do you really want to shutdown the device
            {{ device.metadata.name }}?</span
          >
        </q-card-section>

        <q-card-actions align="right">
          <q-btn flat label="Cancel" color="primary" v-close-popup />
          <q-btn
            flat
            label="Shutdown"
            color="primary"
            v-close-popup
            @click="shutdown"
          />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </div>
</template>

<script lang="ts">
import { computed, defineComponent, reactive, toRefs, ref } from 'vue';
import { useDeviceStore } from 'src/stores/resources';
import apiclient from 'src/k8sclient';
import {
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1alpha1_Device as Device,
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1alpha1_DeviceSpec as DeviceSpec,
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1alpha1_DeviceToken as DeviceToken,
} from 'src/gen';
import DeviceSelect from 'src/components/DeviceSelect.vue';
import { useQuasar } from 'quasar';
import { catchError, info } from 'src/notify';

function serverJoinTokenRequestURL(serverAddress: string) {
  const addrRegex = new RegExp('https://([^/]+)');
  const m = window.location.href.match(addrRegex);
  const addr = m ? m[1] : '';
  return `${serverAddress}/#/setup/request-join-token/${addr}`;
}

function joinTokenNameForServer(serverAddress: string): string {
  return `srv-${serverAddress
    .toLowerCase()
    .replace(/^https?:\/\/([a-z0-9\._-]+)/, '$1')}`
    .replace(/[^a-z0-9]+/, '-')
    .replace(/[^a-z0-9]$/, '');
}

const kc = new apiclient.KubeConfig();
const client = kc.newClient<DeviceToken>(
  '/apis/kubemate.mgoltzsche.github.com/v1alpha1',
  'devicetokens'
);

export default defineComponent({
  name: 'DeviceDetails',
  components: { DeviceSelect },
  props: {
    deviceName: {
      type: String,
      required: true,
    },
  },
  setup(props) {
    const deviceStore = useDeviceStore();
    const deviceSpec = ref<DeviceSpec>({
      mode: DeviceSpec.mode.SERVER,
    });
    const confirmShutdown = ref(false);
    deviceStore.sync(() => {
      const d = deviceStore.resources.find(
        (d) => d.metadata.name == props.deviceName
      );
      if (d) {
        deviceSpec.value = d.spec;
      }
    });
    const quasar = useQuasar();

    async function joinServer(d: Device) {
      const serverAddress = d.spec.serverAddress;
      if (!serverAddress) return;
      const joinTokenName = joinTokenNameForServer(serverAddress);
      d.spec.joinTokenName = joinTokenName;
      console.log(
        `switching device ${d.metadata.name} to ${d.spec.mode} mode, joining ${serverAddress}`
      );
      try {
        await deviceStore.client.update(d);
        try {
          await client.get(joinTokenName);
          console.log(
            `join token for server ${serverAddress} already exists on agent device ${props.deviceName}`
          );
        } catch (e) {
          const url = serverJoinTokenRequestURL(serverAddress);
          console.log(
            `join token for server ${serverAddress} does not exist - redirecting user to token retrieval flow at ${url}`
          );
          window.location.href = url;
        }
      } catch (e: any) {
        quasar.notify({
          type: 'negative',
          message: e.body?.message
            ? `${e.message}: ${e.body?.message}`
            : e.message,
        });
      }
    }

    async function hostServer(d: Device) {
      d.spec.mode = DeviceSpec.mode.SERVER;
      console.log(`switching device ${d.metadata.name} to ${d.spec.mode} mode`);
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
    }

    const state = reactive({
      synchronizing: deviceStore.synchronizing,
      currentDeviceName: computed(
        () =>
          deviceStore.resources.find((d) => d.status.current)?.metadata.name ||
          ''
      ),
      device: computed(() =>
        deviceStore.resources.find((d) => d.metadata.name == props.deviceName)
      ),
      availableDeviceModes: [
        { label: 'Server', value: DeviceSpec.mode.SERVER },
        { label: 'Agent', value: DeviceSpec.mode.AGENT },
      ],
      deleteJoinToken: () => {
        const serverAddress = deviceSpec.value.serverAddress;
        if (!serverAddress) return;
        const joinTokenName = joinTokenNameForServer(serverAddress);
        catchError(
          client.delete(joinTokenName).then(() => {
            const msg = `Deleted cluster join token for server ${serverAddress}.`;
            console.log(msg);
            info(msg);
          })
        );
      },
      apply: async () => {
        var d = deviceStore.resources.find(
          (d) => d.metadata.name == props.deviceName
        );
        if (!d) {
          quasar.notify({
            type: 'negative',
            message: `Device ${props.deviceName} not found!`,
          });
          return;
        }
        d = {
          metadata: d.metadata,
          spec: deviceSpec.value,
          status: d.status,
        };
        switch (d.spec.mode) {
          case DeviceSpec.mode.AGENT:
            await joinServer(d);
            break;
          case DeviceSpec.mode.SERVER:
            await hostServer(d);
            break;
          default:
            console.log(`ERROR: unsupported device mode: ${d.spec.mode}`);
        }
      },
      switchDevice: () => {
        const a = deviceStore.resources.find(
          (d) => d.metadata.name == props.deviceName
        )?.status.address;
        if (a) window.location.href = `${a}/#/devices/${props.deviceName}`;
      },
      requestShutdown: () => {
        confirmShutdown.value = true;
      },
      shutdown: () => {
        const d = deviceStore.resources.find(
          (d) => d.metadata.name == props.deviceName
        );
        if (!d) return;
        catchError(
          deviceStore.client.createSubresource(d, 'shutdown', {}).then(() => {
            info(`Terminating device ${d.metadata.name} ...`);
          })
        );
      },
    });
    return {
      deviceSpec,
      confirmShutdown,
      ...toRefs(state),
    };
  },
});
</script>
