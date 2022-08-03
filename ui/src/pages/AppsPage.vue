<template>
  <div class="row">
    <app-list :class="className" />
    <div class="col-xs-12 col-sm-7 col-md-8">
      <router-view :key="$route.fullPath" />
    </div>
  </div>
</template>

<script lang="ts">
import { computed, defineComponent, toRefs } from 'vue';
import AppList from 'src/components/AppList.vue';
import { useRoute } from 'vue-router';

export default defineComponent({
  name: 'AppsPage',
  components: { AppList },
  setup() {
    const state = {
      className: computed(() =>
        useRoute().params.device ? 'col-xs-0 col-sm' : 'col-xs-12 col-sm'
      ),
    };
    return {
      ...toRefs(state),
    };
  },
});
</script>
