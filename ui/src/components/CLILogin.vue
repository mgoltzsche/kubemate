<template>
  <q-card class="my-card">
    <q-card-section v-if="!script">loading ...</q-card-section>
    <q-card-section v-if="script">
      <p class="text-h6">Kubernetes client configuration</p>
      To connect a Kubernetes client CLI or a
      <a href="https://k8slens.dev/" target="_blank">desktop client</a> to this
      cluster, install
      <a href="https://kubernetes.io/docs/tasks/tools/#kubectl" target="_blank"
        >kubectl</a
      >
      and run the following commands within a terminal:
    </q-card-section>
    <q-card-section class="bg-grey-8 text-white" v-if="script">
      <pre style="overflow-y: auto">{{ script }}</pre>
    </q-card-section>
    <q-card-actions vertical align="center" v-if="script">
      <q-btn
        flat
        ripple
        color="primary"
        icon="content_copy"
        @click="copyToClipboard"
        class="full-width"
        >Copy to clipboard</q-btn
      >
    </q-card-actions>
    <q-space />
    <q-card-section v-if="script"
      ><p class="text-h6">CLI usage examples</p>
      <p>This section shows how to inspect the cluster.</p>
      <q-space />
      List available resource types:</q-card-section
    >
    <q-card-section class="bg-grey-8 text-white" v-if="script">
      <pre style="overflow-y: auto">kubectl api-resources</pre>
    </q-card-section>
    <q-card-section v-if="script"
      >List resources: the device, cluster nodes as well as Pods in all
      namespaces:</q-card-section
    >
    <q-card-section class="bg-grey-8 text-white" v-if="script">
      <pre style="overflow-y: auto">kubectl get device,nodes,pods -Ao wide</pre>
    </q-card-section>
    <q-card-section v-if="script"
      >Inspect a Pod within the kubemate namespace:</q-card-section
    >
    <q-card-section class="bg-grey-8 text-white" v-if="script">
      <pre style="overflow-y: auto">
kubectl describe pod snapcast-client-zpt9d -n kubemate</pre
      >
    </q-card-section>
    <q-card-section v-if="script"
      >Stream the logs of the snapclient container:</q-card-section
    >
    <q-card-section class="bg-grey-8 text-white" v-if="script">
      <pre style="overflow-y: auto">
kubectl logs -f snapcast-client-zpt9d snapclient -n kubemate</pre
      >
    </q-card-section>
  </q-card>
</template>

<script lang="ts">
import apiclient from 'src/k8sclient';
import {
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1alpha1_Certificate as Certificate,
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1alpha1_Device as Device,
} from 'src/gen';
import { defineComponent, ref } from 'vue';
import { error, info } from 'src/notify';
import { copyToClipboard } from 'quasar';
import { useDeviceStore } from 'src/stores/resources';

const kc = new apiclient.KubeConfig();
const certClient = kc.newClient<Certificate>(
  '/apis/kubemate.mgoltzsche.github.com/v1alpha1',
  'certificates'
);
const caCert = ref<Certificate | undefined>();
const device = ref<Device | undefined>();
const script = ref<string>('');

function generateScript() {
  const cluster = device.value?.metadata.name;
  // TODO: avoid exposing the token within the history
  script.value = `printf 'Enter API token:' &&
read -rs K8S_TOKEN &&
kubectl config set-cluster ${cluster} --server=${device.value?.status.address} &&
kubectl config set clusters.${cluster}.certificate-authority-data ${caCert.value?.spec.caCert} &&
kubectl config set-credentials ${cluster} --token="$K8S_TOKEN" &&
kubectl config set-context ${cluster} --cluster=${cluster} --user=${cluster} --namespace=kubemate &&
kubectl config use-context ${cluster}`;
}

export default defineComponent({
  name: 'CLILogin',
  setup() {
    const devices = useDeviceStore();
    devices.sync(() => {
      device.value = devices.resources.find((d) => d.status.current);
      generateScript();
    });
    certClient
      .get('self')
      .then((cert) => {
        caCert.value = cert;
        generateScript();
      })
      .catch((e) => {
        script.value = '';
        error(e);
      });
    return {
      script,
      copyToClipboard() {
        if (script.value) {
          copyToClipboard(script.value)
            .then(() => info('Successfully copied to clipboard!'))
            .catch((e) => error(e));
        } else {
          error(new Error('Script not yet loaded! Please try again.'));
        }
      },
    };
  },
});
</script>
