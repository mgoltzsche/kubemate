<template>
  <q-page class="items-center justify-evenly">
    <wifi-settings />
  </q-page>
</template>

<script lang="ts">
import { defineComponent } from 'vue';
import WifiSettings from 'src/components/WifiSettings.vue';

export default defineComponent({
  name: 'WifiSettingsPage',
  components: { WifiSettings },
});
</script>
