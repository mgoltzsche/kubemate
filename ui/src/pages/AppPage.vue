<template>
  <q-page class="items-center justify-evenly">
    <app-details :app-name="app" />
  </q-page>
</template>

<script lang="ts">
import { computed, defineComponent, toRefs } from 'vue';
import AppDetails from 'src/components/AppDetails.vue';
import { useRoute } from 'vue-router';

export default defineComponent({
  name: 'DevicePage',
  components: { AppDetails },
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
