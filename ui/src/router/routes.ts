import { RouteRecordRaw } from 'vue-router';

const routes: RouteRecordRaw[] = [
  {
    name: 'home',
    path: '/',
    component: () => import('layouts/MainLayout.vue'),
    children: [{ path: '', component: () => import('pages/IndexPage.vue') }],
  },
  {
    name: 'devices',
    path: '/devices',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      {
        name: 'devices',
        path: '',
        component: () => import('pages/DevicesPage.vue'),
        children: [
          {
            name: 'device',
            path: ':name',
            component: () => import('pages/DevicePage.vue'),
          },
        ],
      },
    ],
  },
  {
    name: 'networkinterfaces',
    path: '/networkinterfaces',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      {
        name: 'networkinterfaces',
        path: '',
        component: () => import('pages/NetworkInterfacesPage.vue'),
        children: [
          {
            name: 'networkinterface',
            path: ':name',
            component: () => import('pages/NetworkInterfacePage.vue'),
          },
        ],
      },
    ],
  },
  {
    name: 'apps',
    path: '/apps',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      {
        name: 'apps',
        path: '',
        component: () => import('pages/AppsPage.vue'),
        children: [
          {
            name: 'app',
            path: ':name',
            component: () => import('pages/AppPage.vue'),
          },
        ],
      },
    ],
  },

  {
    name: 'request-join-token',
    path: '/setup/request-join-token/:agent',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      { path: '', component: () => import('pages/RequestJoinTokenPage.vue') },
    ],
  },
  {
    name: 'setup',
    path: '/setup',
    component: () => import('layouts/IframeLayout.vue'),
    children: [
      {
        name: 'accept-join-token',
        path: '/setup/accept-join-token/:server/:token',
        component: () => import('pages/AcceptJoinTokenPage.vue'),
      },
    ],
  },

  // Always leave this as last one,
  // but you can also remove it
  {
    path: '/:catchAll(.*)*',
    component: () => import('pages/ErrorNotFound.vue'),
  },
];

export default routes;
