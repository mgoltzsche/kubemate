import { request } from './request';
import { CancelablePromise } from './CancelablePromise';
import { WatchEvent, watch } from './watch';
import { Resource, ResourceList } from './model';

export class KubeConfig {
  newClient<T extends Resource>(resourceUrl: string): ApiClient<T> {
    return new ApiClient<T>(resourceUrl);
  }
}

export class ApiClient<T extends Resource> {
  private resourceUrl: string;
  constructor(resourceUrl: string) {
    this.resourceUrl = resourceUrl;
  }
  public list(): CancelablePromise<ResourceList<T>> {
    return request({
      method: 'GET',
      url: `${this.resourceUrl}`,
      query: {
        allowWatchBookmarks: 1,
        timeoutSeconds: 10,
      },
    });
  }
  public watch(
    handler: (evt: WatchEvent<T>) => void,
    resourceVersion?: string
  ): void {
    watch(
      `${this.resourceUrl}?watch=1&resourceVersion=${resourceVersion}`,
      handler
    );
  }
  public update(obj: T): CancelablePromise<T> {
    return request({
      method: 'PUT',
      url: `${this.resourceUrl}/${obj.metadata.name || ''}`,
      query: {
        timeoutSeconds: 10,
      },
      body: obj,
    });
  }
}
