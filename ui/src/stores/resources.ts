import {
  defineStore,
  StateTree,
  StoreDefinition,
  _ActionsTree,
  _GettersTree,
  _StoreWithState,
} from 'pinia';
import {
  com_github_mgoltzsche_kubemate_pkg_apis_apps_v1alpha1_App as App,
  com_github_mgoltzsche_kubemate_pkg_apis_devices_v1_Device as Device,
  io_k8s_api_networking_v1_Ingress as Ingress,
  io_k8s_apiextensions_apiserver_pkg_apis_apiextensions_v1_CustomResourceDefinition as CustomResourceDefinition,
} from 'src/gen';
import { Resource } from 'src/k8sclient';
import apiclient from 'src/k8sclient';
import { Ref, ref } from 'vue';
import { ApiClient } from 'src/k8sclient/apiclient';
import sync from './sync';
//import { CustomResource } from 'src/k8sclient/model';

interface ResourceStoreState<T> {
  resources: Ref<T[]>;
  synchronized: Ref<boolean>;
  synchronizing: Ref<boolean>;
}

interface ResourceStoreGetters<T extends Resource> {
  client(): ApiClient<T>;
}

interface ResourceStoreActions<T> {
  sync(): void;
}

type ResourceStoreDefinition<T extends Resource> = StoreDefinition<
  string,
  ResourceStoreState<T>,
  ResourceStoreGetters<T>,
  ResourceStoreActions<T>
>;

const kc = new apiclient.KubeConfig();

function defineResourceStore<T extends Resource>(
  apiVersion: string,
  resource: string
): ResourceStoreDefinition<T> {
  const synchronized = ref(false);
  const synchronizing = ref(false);
  const resources = ref<T[]>([]) as Ref<T[]>;
  const client = kc.newClient<T>(`${apiVersion}/${resource}`);
  const store = defineStore(resource, {
    state: (): ResourceStoreState<T> => ({
      synchronized: synchronized,
      synchronizing: synchronizing,
      resources: resources, // See https://github.com/vuejs/pinia/discussions/973
    }),
    getters: {
      client: () => client,
    },
    actions: {
      sync() {
        if (!this.synchronized && !this.synchronizing) {
          this.synchronizing = true;
          sync(client, resources, synchronized, synchronizing);
        }
      },
    },
  });
  return store;
}

export const useDeviceStore = defineResourceStore<Device>(
  '/apis/kubemate.mgoltzsche.github.com/v1',
  'devices'
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

/*const stores: Record<string, unknown> = {};

function memoize<T>(
  factory: (apiVersion: string, resource: string) => T
): (apiVersion: string, resource: string) => T {
  const key = `${apiVersion}/${resource}`;
  const store = customResourceStores[key];
  if (store) {
    return (apiVersionstore;
  }
  return factory;
}

export function useCustomResourceStore(apiVersion: string, resource: string) {
  return defineResourceStore<CustomResource>(`/apis/${apiVersion}`, resource);
}


const customResourceStores: Record<
  string,
  ResourceStoreDefinition<CustomResource>
> = {};

export function useCustomResourceStore(
  apiVersion: string,
  resource: string
): ResourceStoreDefinition<CustomResource> {
  const key = `${apiVersion}/${resource}`;
  let store = customResourceStores[key];
  if (store) {
    return store;
  }
  store = defineResourceStore<CustomResource>(`/apis/${apiVersion}`, resource);
  customResourceStores[key] = store;
  return store;
}
*/
