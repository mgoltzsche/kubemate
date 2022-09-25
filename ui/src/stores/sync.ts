import { Resource } from 'src/k8sclient';
import { ApiClient } from 'src/k8sclient/apiclient';
import { error } from 'src/notify';
import { CancelablePromise } from 'src/k8sclient/CancelablePromise';
import { EventType } from 'src/k8sclient/watch';
//import { Ref } from 'vue';

interface Ref<T> {
  value: T;
}

export default function sync<T extends Resource>(
  client: ApiClient<T>,
  resources: Ref<T[]>,
  synchronized?: Ref<boolean>,
  synchronizing?: Ref<boolean>
  //callback: (resources: T[]) => void
): CancelablePromise<void> {
  const synchronizedRef = synchronized ? synchronized : { value: false };
  const synchronizingRef = synchronizing ? synchronizing : { value: false };
  synchronizedRef.value = false;
  synchronizingRef.value = true;
  return new CancelablePromise(async (resolve, _, onCancel) => {
    while (true) {
      try {
        if (onCancel.isCancelled) return;
        const list = await client.list();
        if (onCancel.isCancelled) return;
        //const resources = list.items;
        //callback(resources);
        resources.value = list.items;
        synchronizedRef.value = true;
        synchronizingRef.value = false;
        const watch = client.watch((evt) => {
          console.log('sync: received event:', evt);
          switch (evt.type) {
            case EventType.ADDED:
              resources.value.push(evt.object);
              //callback(resources);
              break;
            case EventType.MODIFIED:
              const i = resources.value.findIndex(
                (o) => o.metadata?.name === evt.object.metadata?.name
              );
              if (i >= 0) {
                resources.value[i] = evt.object;
                //callback(resources);
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
                //callback(resources);
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
        synchronizingRef.value = true;
        await new Promise((r) => setTimeout(r, 5000));
        console.log(`sync: restarting ${client.resource()} synchronization`);
      }
    }
    console.log(`sync: terminated ${client.resource()} synchronization`);
  });
}
