<template>
  <q-list>
    <q-item
      clickable
      tag="a"
      :href="link.url"
      :title="link.info"
      :key="`${link.key}`"
      v-for="link in appLinks"
    >
      <q-item-section avatar>
        <q-icon
          :name="`img:${link.url}${link.iconPath}`"
          v-if="link.iconPath"
        />
      </q-item-section>
      <q-item-section>
        <q-item-label>{{ link.title }}</q-item-label>
        <q-item-label caption>{{ link.caption }}</q-item-label>
      </q-item-section>
    </q-item>
  </q-list>
</template>

<script lang="ts">
import { useIngressStore } from 'src/stores/resources';
import { computed, defineComponent, reactive, toRefs } from 'vue';
import { appLinks } from 'src/stores/queries';

function useAppLinks() {
  const store = useIngressStore();
  store.sync();
  const state = reactive({
    appLinks: computed(() => appLinks(store.resources)),
  });
  return {
    ...toRefs(state),
  };
}

export default defineComponent({
  name: 'AppLauncher',
  setup() {
    return { ...useAppLinks() };
  },
});
</script>
