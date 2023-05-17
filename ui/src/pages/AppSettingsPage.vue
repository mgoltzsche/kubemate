<template>
  <q-page class="items-center justify-evenly">
    <app-settings :app="app" />
  </q-page>
</template>

<script lang="ts">
import { computed, defineComponent, toRefs } from 'vue';
import AppSettings from 'src/components/AppSettings.vue';
import { useRoute } from 'vue-router';

export default defineComponent({
  name: 'AppSettingsPage',
  components: { AppSettings },
  setup() {
    const state = {
      app: computed(() => useRoute().params.name as string),
    };
    return {
      ...toRefs(state),
    };
  },
});
</script>
