<template>
  <q-list>
    <q-item
      v-for="device in devices"
      :key="device.metadata.id"
      clickable
      v-ripple
      :to="`/devices/${device.metadata.name}`"
    >
      <q-item-section avatar>
        <q-avatar :color="info" text-color="white"> </q-avatar>
      </q-item-section>
      <q-item-section>
        <q-item-label lines="1">{{ device.metadata.name }}</q-item-label>
        <q-item-label caption lines="1"
          >{{ device.spec.mode }}, {{ device.status.state }}</q-item-label
        >
      </q-item-section>
    </q-item>
  </q-list>
</template>

<script lang="ts">
import { defineComponent } from 'vue';
import { storeToRefs } from 'pinia';
import { useDeviceStore } from 'src/stores/resource-store';

export default defineComponent({
  name: 'DeviceList',
  setup() {
    const store = useDeviceStore();
    store.sync();
    const { resources } = storeToRefs(store);
    return { devices: resources };
  },
});
</script>
