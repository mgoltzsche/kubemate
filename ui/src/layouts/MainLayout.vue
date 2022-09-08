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
          <q-list>
            <q-item
              clickable
              tag="a"
              :href="ingress.url"
              :title="ingress.info"
              :key="`${ingress.key}`"
              v-for="ingress in ingresses"
            >
              <q-item-section avatar>
                <q-icon
                  :name="`img:${ingress.url}${ingress.iconPath}`"
                  v-if="ingress.iconPath"
                />
              </q-item-section>
              <q-item-section>
                <q-item-label>{{ ingress.title }}</q-item-label>
                <q-item-label caption>{{ ingress.caption }}</q-item-label>
              </q-item-section>
            </q-item>
          </q-list>
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
            v-for="link in mainLinks"
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
import EssentialLink from 'components/EssentialLink.vue';
import { version } from '../../package.json';
import {
  useCustomResourceDefinitionStore,
  useDeviceStore,
  useIngressStore,
} from 'src/stores/resources';
import LoginDialog from 'src/components/LoginDialog.vue';
import { io_k8s_api_networking_v1_Ingress as Ingress } from 'src/gen';

const linksList = [
  {
    title: 'Devices & Clusters',
    caption: 'devices',
    icon: 'hub',
    link: '#/devices',
  },
  {
    title: 'Manage Apps',
    caption: 'apps',
    icon: 'extension',
    link: '#/apps',
  },
  {
    title: 'Wifi',
    caption: 'Hotspot',
    icon: 'wifi',
    link: '#/wifi',
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

function useIngresses() {
  const store = useIngressStore();
  store.sync();
  const state = reactive({
    ingresses: computed(() =>
      store.resources
        .map((ing: Ingress) => {
          const a = ing.metadata?.annotations;
          const title = a ? a['kubemate.mgoltzsche.github.com/nav-title'] : '';
          const key = `${ing.metadata?.namespace}/${ing.metadata?.name}`;
          const url = ing.spec?.rules?.find((r) => {
            return !r.host && r.http?.paths && r.http.paths.length > 0;
          })?.http?.paths[0].path;
          return {
            key: key,
            title: title ? title : key,
            caption: title ? key : '',
            info: title ? `${title} (${key})` : key,
            url: url,
            iconPath: a ? a['kubemate.mgoltzsche.github.com/nav-icon'] : '',
          };
        })
        .filter((ing) => ing.url)
    ),
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
      mainLinks: linksList,
      ...useLeftDrawerToggle(),
      ...useLoginDialog(),
      ...useDeviceName(),
      ...useIngresses(),
      ...useCustomResourceDefinitions(),
      version,
    };
  },
});
</script>
