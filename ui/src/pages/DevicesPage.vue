<template>
  <div class="row">
    <device-list :class="className" />
    <div class="col-xs-12 col-sm-7 col-md-8">
      <router-view :key="$route.fullPath" />
    </div>
  </div>
</template>

<script lang="ts">
import { computed, defineComponent, toRefs } from 'vue';
import DeviceList from 'src/components/DeviceList.vue';
import { useRoute } from 'vue-router';

export default defineComponent({
  name: 'DevicesPage',
  components: { DeviceList },
  setup() {
    const state = {
      className: computed(() =>
        useRoute().params.name ? 'col-xs-0 col-sm gt-xs' : 'col-xs-12 col-sm'
      ),
    };
    return {
      ...toRefs(state),
    };
  },
});
</script>
