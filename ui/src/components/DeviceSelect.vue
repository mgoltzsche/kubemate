<template>
  <q-select
    filled
    :model-value="address"
    use-input
    hide-selected
    fill-input
    input-debounce="0"
    :options="availableServers"
    @filter="filterFn"
    @input-value="setServer"
    hint="Device name or URL to connect with"
    placeholder="https://my-machine"
  />
</template>

<script lang="ts">
import { computed, defineComponent, reactive, toRefs, ref } from 'vue';
import { useDeviceDiscoveryStore } from 'src/stores/resources';
import { com_github_mgoltzsche_kubemate_pkg_apis_devices_v1alpha1_DeviceSpec as DeviceSpec } from 'src/gen';

export default defineComponent({
  name: 'DeviceSelect',
  props: {
    modelValue: {
      type: String,
    },
  },
  emits: ['update:modelValue'],
  setup(props, context) {
    const discoveryStore = useDeviceDiscoveryStore();
    const address = ref(props.modelValue);

    const needle = ref('');
    const state = reactive({
      address,
      availableServers: computed(() => {
        return discoveryStore.resources
          .filter(
            (d) =>
              !d.spec.current &&
              d.spec.mode == DeviceSpec.mode.SERVER &&
              d.spec.address &&
              d.spec.address.indexOf(needle.value) >= 0
          )
          .map((d) => ({ label: d.metadata.name, value: d.spec.address }));
      }),
      filterFn(val: string, update: (_: () => void) => void) {
        update(() => {
          needle.value = val.toLocaleLowerCase();
        });
      },
      setServer(addr: string) {
        if (addr && !addr.startsWith('https://')) {
          addr = `https://${addr}`;
        }
        address.value = addr;
        context.emit('update:modelValue', addr);
      },
    });
    return {
      ...toRefs(state),
    };
  },
});
</script>
