<template>
  <q-page class="column" style="max-width: 1000px; margin: 0 auto">
    <div class="col q-pa-md" v-if="!ready">
      <div class="row justify-around">
        <div class="col-2 q-pa-md" v-for="index in 2" :key="index">
          <q-skeleton type="rect" />
        </div>
      </div>
      <div class="row">
        <q-item
          class="col-12 col-sm-6 col-md-4 q-py-lg"
          v-for="index in 3"
          :key="index"
        >
          <q-item-section avatar>
            <q-skeleton type="QAvatar" />
          </q-item-section>
          <q-item-section>
            <q-item-label>
              <q-skeleton type="text" style="width: 55%" />
            </q-item-label>
            <q-item-label caption>
              <q-skeleton type="text" style="width: 75%" />
            </q-item-label>
          </q-item-section>
        </q-item>
      </div>
    </div>
    <div class="col-auto q-pa-md" v-if="ready">
      <q-tabs
        v-model="tab"
        dense
        narrow-indicator
        inline-label
        class="text-primary"
        active-color="primary"
        indicator-color="primary"
        align="justify"
      >
        <q-tab name="apps" label="Apps" icon="apps" />
        <q-tab name="settings" label="Settings" icon="settings" />
      </q-tabs>
      <q-separator />
    </div>

    <q-tab-panels v-model="tab" animated swipeable class="col" v-if="ready">
      <q-tab-panel name="apps" class="no-padding-x-xs">
        <div class="q-pa-md text-center" v-if="appLinks.length === 0">
          No apps installed.
        </div>
        <q-list class="row full-width" v-if="appLinks.length > 0">
          <q-item
            class="col-12 col-sm-6 col-md-4 q-py-md k-pa"
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
                class="k-icon"
                v-if="link.iconPath"
              />
            </q-item-section>
            <q-item-section>
              <q-item-label>{{ link.title }}</q-item-label>
              <q-item-label caption>{{ link.caption }}</q-item-label>
            </q-item-section>
          </q-item>
        </q-list>
      </q-tab-panel>

      <q-tab-panel name="settings">
        <q-list style="max-width: 350px; margin: 0 auto" class="q-py-md">
          <EssentialLink
            v-bind="link"
            v-for="link in mainLinks"
            :key="link.title"
          />
        </q-list>
      </q-tab-panel>
    </q-tab-panels>
  </q-page>
</template>

<style lang="scss">
.k-pa {
  padding: 20px 24px;
  @media (max-width: $breakpoint-xs-max) {
    padding: 14px 28px;
  }
}
.k-icon > img {
  width: 48px;
  height: 48px;
}
.no-padding-x-xs {
  @media (max-width: $breakpoint-xs-max) {
    padding-left: 0 !important;
    padding-right: 0 !important;
  }
}
</style>

<script lang="ts">
import { computed } from '@vue/reactivity';
import EssentialLink from 'components/EssentialLink.vue';
import { useIngressStore } from 'src/stores/resources';
import { defineComponent, reactive, ref, toRefs } from 'vue';
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
    ready: computed(() => store.synchronized),
  });
  return {
    ...toRefs(state),
  };
}

export default defineComponent({
  name: 'IndexPage',
  components: {
    EssentialLink,
  },
  setup() {
    const store = useIngressStore();
    const tab = ref('apps');
    store.sync(() => {
      tab.value = appLinks(store.resources).length > 0 ? 'apps' : 'settings';
    });
    return {
      mainLinks,
      tab: tab,
      ...useAppLinks(),
    };
  },
});
</script>
