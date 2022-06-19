<template>
  <q-page class="row items-center justify-evenly">
    <example-component
      title="Example component"
      active
      :todos="todos"
      :meta="meta"
    ></example-component>
  </q-page>
</template>

<script lang="ts">
import { Todo, Meta } from 'components/models';
import ExampleComponent from 'components/ExampleComponent.vue';
import { defineComponent, ref } from 'vue';
import client from 'src/k8sclient';
import { com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_Device as Device } from 'src/gen';

const kc = new client.KubeConfig();
const c = kc.newClient<Device>(
  '/apis/kubemate.mgoltzsche.github.com/v1/devices'
);
c.list().then((list) => {
  list.items.forEach((item) => {
    console.log('DEVICE', item);
  });
  c.watch((evt) => {
    console.log('EVENT ' + evt.type, evt.object);
  }, list.metadata.resourceVersion || '');
});

export default defineComponent({
  name: 'IndexPage',
  components: { ExampleComponent },
  setup() {
    const todos = ref<Todo[]>([
      {
        id: 1,
        content: 'ct1',
      },
      {
        id: 2,
        content: 'ct2',
      },
      {
        id: 3,
        content: 'ct3',
      },
      {
        id: 4,
        content: 'ct4',
      },
      {
        id: 5,
        content: 'ct5',
      },
    ]);
    const meta = ref<Meta>({
      totalCount: 1200,
    });
    return { todos, meta };
  },
});
</script>
