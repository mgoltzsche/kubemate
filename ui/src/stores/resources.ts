import { defineStore } from 'pinia';
import {
  com_github_mgoltzsche_kubemate_pkg_apis_apps_v1alpha1_App as App,
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_Device as Device,
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_DeviceDiscovery as DeviceDiscovery,
  io_k8s_api_networking_v1_Ingress as Ingress,
  io_k8s_apiextensions_apiserver_pkg_apis_apiextensions_v1_CustomResourceDefinition as CustomResourceDefinition,
} from 'src/gen';
import { Resource } from 'src/k8sclient';
import apiclient from 'src/k8sclient';
import { Ref, ref } from 'vue';
import sync from './sync';

const kc = new apiclient.KubeConfig();

function defineResourceStore<T extends Resource>(
  apiVersion: string,
  resource: string
) {
  const resources = ref<T[]>([]) as Ref<T[]>;
  const synchronizing = ref(false);
  const client = kc.newClient<T>(apiVersion, resource);
  let initializers = [] as (() => void)[];
  return defineStore(resource, {
    state: () => ({
      synchronized: false,
      synchronizing: synchronizing,
      resources: resources, // See https://github.com/vuejs/pinia/discussions/973
    }),
    getters: {
      client: () => client,
    },
    actions: {
      sync(fn?: () => void) {
        if (!this.synchronized && !this.synchronizing) {
          sync(client, resources, synchronizing, () => {
            this.synchronized = true;
            initializers.forEach((fn) => {
              fn();
            });
            initializers = [];
          });
        }
        if (fn) {
          if (this.synchronized) {
            fn();
            return;
          }
          initializers.push(fn);
        }
      },
    },
  });
}

export const useDeviceStore = defineResourceStore<Device>(
  '/apis/kubemate.mgoltzsche.github.com/v1',
  'devices'
);

export const useDeviceDiscoveryStore = defineResourceStore<DeviceDiscovery>(
  '/apis/kubemate.mgoltzsche.github.com/v1',
  'devicediscovery'
);

export const useAppStore = defineResourceStore<App>(
  '/apis/apps.kubemate.mgoltzsche.github.com/v1alpha1',
  'apps'
);

export const useIngressStore = defineResourceStore<Ingress>(
  '/apis/networking.k8s.io/v1',
  'ingresses'
);

export const useCustomResourceDefinitionStore =
  defineResourceStore<CustomResourceDefinition>(
    '/apis/apiextensions.k8s.io/v1',
    'customresourcedefinitions'
  );
