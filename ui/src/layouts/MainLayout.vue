<template>
  <q-layout view="lHh Lpr lFf">
    <q-header elevated>
      <q-toolbar>
        <q-btn
          flat
          dense
          round
          icon="menu"
          aria-label="Menu"
          @click="toggleLeftDrawer"
        />

        <q-toolbar-title> Kubemate </q-toolbar-title>

        <q-btn
          flat
          dense
          round
          icon="login"
          aria-label="Login"
          @click="loginDialogOpen = true"
        />
        <div>Kubemate {{ version }} @ {{ deviceName }}</div>
      </q-toolbar>
    </q-header>

    <q-drawer v-model="leftDrawerOpen" show-if-above bordered>
      <q-list>
        <q-item-label header> Essential Links </q-item-label>

        <EssentialLink
          v-for="link in essentialLinks"
          :key="link.title"
          v-bind="link"
        />

        <q-item-label header> Settings </q-item-label>

        <q-item
          clickable
          tag="a"
          :href="`#/customresourcedefinition/${crd.metadata?.name}`"
          :title="crd.metadata?.name"
          :key="crd.metadata?.name"
          v-for="crd in customResourceDefinitions"
        >
          <q-item-section>
            <q-item-label>{{ crd.spec.names.plural }}</q-item-label>
            <q-item-label caption>{{ crd.metadata?.name }}</q-item-label>
          </q-item-section>
        </q-item>
      </q-list>
    </q-drawer>

    <login-dialog v-model="loginDialogOpen" />

    <q-page-container>
      <q-breadcrumbs v-if="$route.path != '/'" class="q-pa-md">
        <q-breadcrumbs-el icon="home" to="/" />
        <q-breadcrumbs-el
          v-for="r in $route.matched.slice(1, $route.matched.length - 1)"
          :key="r.name"
          :label="r.name?.toString()"
          :to="r.path"
        />
        <q-breadcrumbs-el
          :label="$route.params.name?.toString() || $route.name?.toString()"
        />
      </q-breadcrumbs>
      <router-view />
    </q-page-container>
  </q-layout>
</template>

<script lang="ts">
import { defineComponent, ref, toRefs, computed, reactive } from 'vue';
import EssentialLink from 'components/EssentialLink.vue';
import { version } from '../../package.json';
import {
  useCustomResourceDefinitionStore,
  useDeviceStore,
} from 'src/stores/resources';
import LoginDialog from 'src/components/LoginDialog.vue';

const linksList = [
  {
    title: 'Devices & Clusters',
    caption: 'devices',
    icon: 'hub',
    link: '#/devices',
    target: '_self',
  },
  {
    title: 'Apps',
    caption: 'apps',
    icon: 'extension',
    link: '#/apps',
    target: '_self',
  },
  {
    title: 'Docs',
    caption: 'quasar.dev',
    icon: 'school',
    link: 'https://quasar.dev',
    target: '_blank',
  },
  {
    title: 'Github',
    caption: 'github.com/quasarframework',
    icon: 'code',
    link: 'https://github.com/quasarframework',
    target: '_blank',
  },
  {
    title: 'Quasar Awesome',
    caption: 'Community Quasar projects',
    icon: 'favorite',
    link: 'https://awesome.quasar.dev',
    target: '_blank',
  },
];

function useLeftDrawerToggle() {
  const leftDrawerOpen = ref(false);
  return {
    leftDrawerOpen,
    toggleLeftDrawer() {
      leftDrawerOpen.value = !leftDrawerOpen.value;
    },
  };
}

function useLoginDialog() {
  const loginDialogOpen = ref(false);
  return {
    loginDialogOpen,
    toggleLoginDialog() {
      console.log('logindialog', !loginDialogOpen.value);
      loginDialogOpen.value = !loginDialogOpen.value;
    },
  };
}

function useDeviceName() {
  const store = useDeviceStore();
  store.sync();
  const state = reactive({
    deviceName: computed(
      () => store.resources.find((d) => d.status.current)?.metadata.name || ''
    ),
  });
  return {
    ...toRefs(state),
  };
}

function useCustomResourceDefinitions() {
  const store = useCustomResourceDefinitionStore();
  store.sync();
  const state = reactive({
    customResourceDefinitions: computed(() => store.resources),
  });
  return {
    ...toRefs(state),
  };
}

export default defineComponent({
  name: 'MainLayout',

  components: {
    EssentialLink,
    LoginDialog,
  },

  setup() {
    return {
      essentialLinks: linksList,
      ...useLeftDrawerToggle(),
      ...useLoginDialog(),
      ...useDeviceName(),
      ...useCustomResourceDefinitions(),
      version,
    };
  },
});
</script>
