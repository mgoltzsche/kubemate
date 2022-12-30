<template>
  <login-dialog v-model="showLoginDialog" @cancel="cancel" persistent />
  <q-dialog v-model="showConfirmDialog" persistent>
    <q-card>
      <q-card-section class="row items-center">
        Do you want to allow
        <a :href="`https://${agentAddress}`">{{ agentAddress }}</a>
        to join the controller {{ controllerName }} as agent?
      </q-card-section>

      <q-card-actions>
        <q-btn
          flat
          label="Deny"
          color="negative"
          v-close-popup
          @click="cancel"
        />
        <q-btn
          flat
          label="Allow"
          color="positive"
          v-close-popup
          @click="syncJoinTokenToAgent"
        />
      </q-card-actions>
    </q-card>
    <q-dialog v-model="showSyncDialog">
      <q-card
        style="width: 700px; max-width: 80vw; height: 500px; max-height: 80vh"
      >
        <iframe
          height="100%"
          width="100%"
          frameBorder="0"
          :src="`https://${agentAddress}/#/setup/accept-join-token/${controllerName}/${joinToken}`"
        ></iframe>
      </q-card>
    </q-dialog>
  </q-dialog>
</template>

<script lang="ts">
import { useAuthStore } from 'src/stores/auth';
import { defineComponent, reactive, toRefs, ref } from 'vue';
import LoginDialog from 'src/components/LoginDialog.vue';
import { useRoute } from 'vue-router';
import { useDeviceStore } from 'src/stores/resources';
import { computed } from '@vue/reactivity';
import apiclient from 'src/k8sclient';
import { com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_DeviceToken as DeviceToken } from 'src/gen';
import { useQuasar } from 'quasar';

export default defineComponent({
  components: { LoginDialog },
  name: 'RequestJoinTokenPage',
  methods: {
    cancel() {
      window.history.back();
    },
  },
  setup() {
    const auth = useAuthStore();
    const devices = useDeviceStore();
    const route = useRoute();
    const quasar = useQuasar();
    const kc = new apiclient.KubeConfig();
    const client = kc.newClient<DeviceToken>(
      '/apis/kubemate.mgoltzsche.github.com/v1',
      'devicetokens'
    );
    const agent = () => {
      const a = (route.params as any).agent;
      return devices.resources.find((r) => r.metadata.name == a);
    };
    const joinToken = ref('');
    const showSyncDialog = ref(false);
    const state = reactive({
      showLoginDialog: computed(() => !auth.authenticated),
      showConfirmDialog: computed(() => auth.authenticated),
      confirm: false,
      agentName: computed(() => {
        const a = (route.params as any).agent;
        return agent()?.metadata.name || a;
      }),
      agentAddress: computed(() => {
        return (route.params as any).agent;
      }),
      controllerName: computed(
        () => devices.resources.find((r) => r.status.current)?.metadata.name
      ),
      async syncJoinTokenToAgent() {
        try {
          const thisDevice = devices.resources.find((r) => r.status.current);
          if (thisDevice && thisDevice.metadata.name) {
            const t = await client.get(thisDevice.metadata.name);
            if (t.status?.joinToken) {
              joinToken.value = encodeURIComponent(t.status.joinToken);
              showSyncDialog.value = true;
            }
          }
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
      joinToken,
      showSyncDialog,
      ...toRefs(state),
    };
  },
});
</script>
