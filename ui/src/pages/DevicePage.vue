<template>
  <q-page class="items-center justify-evenly">
    <device-details :device-name="device" />
  </q-page>
</template>

<script lang="ts">
import { computed, defineComponent, toRefs } from 'vue';
import DeviceDetails from 'src/components/DeviceDetails.vue';
import { useRoute } from 'vue-router';

export default defineComponent({
  name: 'DevicePage',
  components: { DeviceDetails },
  setup() {
    const state = {
      device: computed(() => useRoute().params.name as string),
    };
    return {
      ...toRefs(state),
    };
  },
});
</script>
