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
        <q-expansion-item
          group="main"
          icon="apps"
          label="Apps"
          header-class="text-primary"
        >
          <app-launcher />
        </q-expansion-item>
        <q-separator />
        <q-expansion-item
          group="main"
          icon="explore"
          label="Custom Resources"
          header-class="text-primary"
        >
          <q-list>
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
        </q-expansion-item>
        <q-separator />
        <q-expansion-item
          group="main"
          icon="settings"
          label="Settings"
          default-opened
          header-class="text-primary"
        >
          <EssentialLink
            v-for="link in settingsLinks"
            :key="link.title"
            v-bind="link"
          />
        </q-expansion-item>
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
import AppLauncher from 'components/AppLauncher.vue';
import EssentialLink from 'components/EssentialLink.vue';
import { version } from '../../package.json';
import {
  useCustomResourceDefinitionStore,
  useDeviceStore,
} from 'src/stores/resources';
import LoginDialog from 'src/components/LoginDialog.vue';

const settingsLinks = [
  {
    title: 'Manage Apps',
    caption: 'apps',
    icon: 'extension',
    link: '#/apps',
  },
  {
    title: 'Devices & Clusters',
    caption: 'devices',
    icon: 'hub',
    link: '#/devices',
  },
  {
    title: 'Network',
    caption: 'Wifi settings & ethernet status',
    icon: 'wifi',
    link: '#/networkinterfaces',
  },
  {
    title: 'API Access',
    caption: 'Connect CLI & desktop clients',
    icon: 'terminal',
    link: '#/cli-login',
  },
  {
    title: 'Source code and issue tracker',
    caption: 'github.com/mgoltzsche/kubemate',
    icon: 'code',
    link: 'https://github.com/mgoltzsche/kubemate',
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
    AppLauncher,
    EssentialLink,
    LoginDialog,
  },

  setup() {
    return {
      settingsLinks: settingsLinks,
      ...useLeftDrawerToggle(),
      ...useLoginDialog(),
      ...useDeviceName(),
      ...useCustomResourceDefinitions(),
      version,
    };
  },
});
</script>
