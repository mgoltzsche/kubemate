import { Resource } from 'src/k8sclient';
import { ApiClient } from 'src/k8sclient/apiclient';
import { error } from 'src/notify';
import { CancelablePromise } from 'src/k8sclient/CancelablePromise';
import { EventType } from 'src/k8sclient/watch';

interface Ref<T> {
  value: T;
}

export default function sync<T extends Resource>(
  client: ApiClient<T>,
  resources: Ref<T[]>,
  loading: Ref<boolean>,
  callback?: (resources: T[]) => void
): CancelablePromise<void> {
  return new CancelablePromise(async (resolve, _, onCancel) => {
    while (true) {
      try {
        if (onCancel.isCancelled) return;
        loading.value = true;
        const list = await client.list();
        loading.value = false;
        if (onCancel.isCancelled) return;
        resources.value = list.items;
        if (callback) callback(list.items);
        const watch = client.watch((evt) => {
          console.log('sync: received event:', evt);
          switch (evt.type) {
            case EventType.ADDED:
              resources.value.push(evt.object);
              if (callback) callback(resources.value);
              break;
            case EventType.MODIFIED:
              const i = resources.value.findIndex(
                (o) => o.metadata?.name === evt.object.metadata?.name
              );
              if (i >= 0) {
                resources.value[i] = evt.object;
                if (callback) callback(resources.value);
              } else {
                console.log(
                  `ERROR: sync: server emitted ${
                    EventType.MODIFIED
                  } event for a resource (${client.resource()}/${
                    evt.object.metadata?.name
                  }) that is not known by the client`
                );
              }
              break;
            case EventType.DELETED:
              const res: T[] = [];
              for (let i = 0; i < resources.value.length; i++) {
                const r = resources.value[i];
                if (r.metadata?.name !== evt.object.metadata?.name) {
                  res.push(r);
                }
              }
              if (res.length != resources.value.length) {
                resources.value = res;
                if (callback) callback(res);
              }
              break;
            case EventType.BOOKMARK:
              break;
            default:
              console.log(`WARN: sync: unsupported event type: ${evt.type}`);
              break;
          }
        }, list.metadata.resourceVersion || '');
        if (onCancel.isCancelled) {
          await watch.cancel();
          break;
        }
        onCancel(async () => {
          await watch.cancel();
        });
        await watch;
        resolve();
        break;
      } catch (e) {
        if (onCancel.isCancelled) {
          console.log('sync: canceled:', e);
          break;
        }
        error(e);
        loading.value = true;
        await new Promise((r) => setTimeout(r, 5000));
        console.log(`sync: restarting ${client.resource()} synchronization`);
      }
    }
    console.log(`sync: terminated ${client.resource()} synchronization`);
    loading.value = false;
  });
}
