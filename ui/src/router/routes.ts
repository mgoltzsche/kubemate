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

  // Always leave this as last one,
  // but you can also remove it
  {
    path: '/:catchAll(.*)*',
    component: () => import('pages/ErrorNotFound.vue'),
  },
];

export default routes;
