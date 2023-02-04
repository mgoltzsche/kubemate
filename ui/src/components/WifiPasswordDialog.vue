<template>
  <q-dialog v-model="open">
    <q-card style="min-width: 350px">
      <q-card-section>
        <div class="text-h6">Wifi password for {{ ssid }}</div>
      </q-card-section>

      <q-card-section class="q-pt-none">
        <q-input
          dense
          autofocus
          v-model="password.password"
          hint="Must have 8..63 characters!"
          :type="showPassword ? 'text' : 'password'"
          @keyup.enter="saveWifiPassword()"
        >
          <template v-slot:append>
            <q-icon
              :name="showPassword ? 'visibility_off' : 'visibility'"
              class="cursor-pointer"
              @click="showPassword = !showPassword"
            />
          </template>
        </q-input>
      </q-card-section>

      <q-card-actions align="right" class="text-primary">
        <q-btn flat label="Cancel" v-close-popup />
        <q-btn
          flat
          label="OK"
          v-on:click="saveWifiPassword()"
          :disable="
            password.password.length < 8 || password.password.length > 63
          "
        />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script lang="ts">
import { defineComponent, ref } from 'vue';
import apiclient from 'src/k8sclient';
import { catchError, info } from 'src/notify';
import { com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_WifiPassword as WifiPassword } from 'src/gen';

const kc = new apiclient.KubeConfig();
const wifiPasswordClient = kc.newClient<WifiPassword>(
  '/apis/kubemate.mgoltzsche.github.com/v1',
  'wifipasswords'
);
const password = ref<WifiConnectPassword>({
  resourceName: '',
  password: '',
});

interface WifiConnectPassword {
  resourceName: string;
  password: string;
}

export default defineComponent({
  name: 'WifiPasswordDialog',
  props: {
    value: {
      // enables v-model (two-way binding) with emitted 'input' event, see https://www.digitalocean.com/community/tutorials/how-to-add-v-model-support-to-custom-vue-js-components
      type: Boolean,
      default: false,
    },
    name: {
      type: String,
      required: true,
    },
    ssid: {
      type: String,
      required: true,
    },
  },
  data() {
    return {
      open: this.value,
      showPassword: false,
    };
  },
  methods: {
    closeDialog() {
      this.$emit('input', false);
    },
    saveWifiPassword() {
      wifiPasswordClient
        .get(password.value.resourceName)
        .then((pw) => {
          if (password.value.password != pw.data.password) {
            pw.data.password = password.value.password;
            catchError(
              wifiPasswordClient.update(pw).then(() => {
                info('Wifi password saved.');
                this.closeDialog();
              })
            );
          } else {
            this.closeDialog();
          }
        })
        .catch(() => {
          catchError(
            wifiPasswordClient
              .create({
                metadata: {
                  name: password.value.resourceName,
                },
                data: {
                  password: password.value.password,
                },
              })
              .then(() => {
                info('Wifi password saved.');
                this.closeDialog();
              })
          );
        });
    },
  },
  setup(props) {
    if (props.name) {
      password.value = {
        resourceName: props.name,
        password: '',
      };
      catchError(
        wifiPasswordClient.get(props.name).then((pw) => {
          password.value = {
            resourceName: props.name,
            password: pw.data.password,
          };
        })
      );
    }
    return {
      password,
    };
  },
});
</script>
