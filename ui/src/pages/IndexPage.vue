<template>
  <q-page class="row items-center justify-evenly">
    <div class="q-pa-md row items-start q-gutter-md">
      <q-card v-if="appLinks.length > 0">
        <q-card-section>
          <div class="text-h6">Apps</div>
        </q-card-section>
        <app-launcher />
      </q-card>
      <q-card>
        <q-card-section>
          <div class="text-h6">Settings</div>
        </q-card-section>
        <q-list>
          <EssentialLink
            v-bind="link"
            v-for="link in mainLinks"
            :key="link.title"
          />
        </q-list>
      </q-card>
    </div>
  </q-page>
</template>

<script lang="ts">
import { computed } from '@vue/reactivity';
import EssentialLink from 'components/EssentialLink.vue';
import AppLauncher from 'components/AppLauncher.vue';
import { useIngressStore } from 'src/stores/resources';
import { defineComponent, reactive, toRefs } from 'vue';
import { appLinks } from 'src/stores/queries';

const mainLinks = [
  {
    title: 'Make kubemate join a wifi network',
    icon: 'wifi',
    link: '#/networkinterfaces',
  },
  {
    title: 'Make kubemate join a cluster',
    icon: 'hub',
    link: '#/devices',
  },
  {
    title: 'Manage kubemate apps',
    icon: 'extension',
    link: '#/apps',
  },
];

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
  name: 'IndexPage',
  components: {
    AppLauncher,
    EssentialLink,
  },
  setup() {
    return {
      mainLinks,
      ...useAppLinks(),
    };
  },
});
</script>
