<template>
  <login-dialog v-model="showLoginDialog" @cancel="cancel" persistent />
  <div
    class="fullscreen text-center q-pa-md flex flex-center"
    v-if="!showLoginDialog"
  >
    <q-card>
      <q-card-section class="row items-center">
        Do you want to accept/replace the join token for server
        {{ $route.params.server }} on device {{ agentName }}?
      </q-card-section>

      <q-card-actions>
        <q-btn
          flat
          label="Cancel"
          color="negative"
          v-close-popup
          @click="cancel"
        />
        <q-btn
          flat
          label="Accept"
          color="positive"
          v-close-popup
          @click="saveJoinToken"
        />
      </q-card-actions>
    </q-card>
  </div>
</template>

<script lang="ts">
import { useQuasar } from 'quasar';
import { useDeviceStore } from 'src/stores/resources';
import { computed, defineComponent, reactive, toRefs } from 'vue';
import { useRoute } from 'vue-router';
import LoginDialog from 'src/components/LoginDialog.vue';
import apiclient from 'src/k8sclient';
import { com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_DeviceToken as DeviceToken } from 'src/gen';
import { useAuthStore } from 'src/stores/auth';

export default defineComponent({
  name: 'AcceptJoinTokenPage',
  components: { LoginDialog },
  setup() {
    const route = useRoute();
    const auth = useAuthStore();
    const devices = useDeviceStore();
    devices.sync();
    const quasar = useQuasar();
    const kc = new apiclient.KubeConfig();
    const client = kc.newClient<DeviceToken>(
      '/apis/kubemate.mgoltzsche.github.com/v1/devicetokens'
    );
    function browseToAgentPage() {
      const d = devices.resources.find((r) => r.status.current);
      if (d && d.status.address)
        window.parent.location.href = `${d.status.address}/#/devices/${d.metadata.name}`;
    }
    const state = reactive({
      showLoginDialog: computed(() => !auth.authenticated),
      agentName: computed(() => {
        return devices.resources.find((r) => r.status.current)?.metadata.name;
      }),
      cancel: browseToAgentPage,
      saveJoinToken: async () => {
        // TODO: avoid having to login again to the agent device - make local storage work within the iframe.
        const d = devices.resources.find((r) => r.status.current);
        if (!d || !d.metadata.name || !route.params.token) return;
        try {
          await client.delete(route.params.server as string);
        } catch (e) {}
        const t: DeviceToken = {
          metadata: {
            name: route.params.server as string,
          },
          data: {
            token: decodeURIComponent(route.params.token as string),
          },
        };
        try {
          await client.create(t);
          browseToAgentPage();
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
    };
  },
});
</script>
