<template>
  <q-list>
    <q-item
      v-for="app in apps"
      :key="`${app.metadata?.namespace}/${app.metadata?.name}`"
      clickable
      v-ripple
      :to="`/apps/${app.metadata?.name}`"
    >
      <q-item-section avatar>
        <q-avatar color="info" text-color="white"> </q-avatar>
      </q-item-section>
      <q-item-section>
        <q-item-label lines="1">{{ app.metadata?.name }}</q-item-label>
        <q-item-label caption lines="1">{{ app.status?.state }}</q-item-label>
      </q-item-section>
    </q-item>
  </q-list>
</template>

<script lang="ts">
import { defineComponent } from 'vue';
import { storeToRefs } from 'pinia';
import { useAppStore } from 'src/stores/resources';

export default defineComponent({
  name: 'AppList',
  setup() {
    const store = useAppStore();
    store.sync();
    const { resources } = storeToRefs(store);
    return { apps: resources };
  },
});
</script>
