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
  io_k8s_apiextensions_apiserver_pkg_apis_apiextensions_v1_CustomResourceDefinition as CustomResourceDefinition,
} from 'src/gen';
import { Resource } from 'src/k8sclient';
import apiclient from 'src/k8sclient';
import { Ref, ref } from 'vue';
import { Notify } from 'quasar';
import { ApiClient } from 'src/k8sclient/apiclient';
//import { CustomResource } from 'src/k8sclient/model';

interface ResourceStoreState<T> {
  synchronizing: boolean;
  resources: Ref<T[]>;
}

interface ResourceStoreGetters<T> {
  client(): ApiClient<T>;
}

interface ResourceStoreActions<T> {
  sync(): void;
  setResources(r: T[]): void;
}

type ResourceStoreDefinition<T> = StoreDefinition<
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
  const client = kc.newClient<T>(`${apiVersion}/${resource}`);
  const store = defineStore(resource, {
    state: (): ResourceStoreState<T> => ({
      synchronizing: false,
      resources: ref<T[]>([]) as Ref<T[]>, // See https://github.com/vuejs/pinia/discussions/973
    }),
    getters: {
      client: () => client,
    },
    actions: {
      sync() {
        if (!this.synchronizing) {
          this.synchronizing = true;
          client
            .list()
            .then((list) => {
              this.setResources(list.items);
              this.synchronizing = false;
              client.watch((evt) => {
                console.log('EVENT ' + evt.type, evt.object);
              }, list.metadata.resourceVersion || '');
            })
            .catch((e) => {
              Notify.create({
                type: 'negative',
                message: e.body?.message
                  ? `${e.message}: ${e.body?.message}`
                  : e.message,
              });
            });
        }
      },
      setResources(resources: T[]) {
        this.resources = [];
        resources.forEach((r) => this.resources.push(r));
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
