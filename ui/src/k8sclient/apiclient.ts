import { request } from './request';
import { CancelablePromise } from './CancelablePromise';
import { WatchEvent, watch } from './watch';
import { Resource, ResourceList } from './model';
import { useAuthStore } from 'src/stores/auth';

export class KubeConfig {
  newClient<T extends Resource>(resourceUrl: string): ApiClient<T> {
    return new ApiClient<T>(resourceUrl);
  }
}

export class ApiClient<T extends Resource> {
  private auth = useAuthStore();
  private resourceUrl: string;
  constructor(resourceUrl: string) {
    this.resourceUrl = resourceUrl;
  }
  public create(obj: T): CancelablePromise<T> {
    console.log(`client: create ${this.resourceUrl}`);
    return request({
      method: 'POST',
      url: this.resourceUrl,
      query: {
        timeoutSeconds: 10,
      },
      headers: {
        Authorization: 'Bearer ' + this.auth.token,
      },
      body: obj,
    });
  }
  public update(obj: T): CancelablePromise<T> {
    console.log(`client: update ${this.resourceUrl}`);
    return request({
      method: 'PUT',
      url: `${this.resourceUrl}/${obj.metadata?.name || ''}`,
      query: {
        timeoutSeconds: 10,
      },
      headers: {
        Authorization: 'Bearer ' + this.auth.token,
      },
      body: obj,
    });
  }
  public delete(
    name: string,
    namespace?: string
  ): CancelablePromise<ResourceList<T>> {
    const url = namespace
      ? `namespaces/${namespace}/${this.resourceUrl}/${name}`
      : `${this.resourceUrl}/${name}`;
    console.log(`client: delete ${url}`);
    return request({
      method: 'DELETE',
      url: url,
      query: {
        timeoutSeconds: 10,
      },
      headers: {
        Authorization: 'Bearer ' + this.auth.token,
      },
    });
  }
  public get(name: string, namespace?: string): CancelablePromise<T> {
    const url = namespace
      ? `namespaces/${namespace}/${this.resourceUrl}/${name}`
      : `${this.resourceUrl}/${name}`;
    console.log(`client: get ${url}`);
    return request({
      method: 'GET',
      url: url,
      query: {
        timeoutSeconds: 10,
      },
      headers: {
        Authorization: 'Bearer ' + this.auth.token,
      },
    });
  }
  public list(): CancelablePromise<ResourceList<T>> {
    console.log(`client: list ${this.resourceUrl}`);
    return request({
      method: 'GET',
      url: `${this.resourceUrl}`,
      query: {
        timeoutSeconds: 10,
      },
      headers: {
        Authorization: 'Bearer ' + this.auth.token,
      },
    });
  }
  public watch(
    handler: (evt: WatchEvent<T>) => void,
    resourceVersion?: string
  ): void {
    console.log(`client: watch ${this.resourceUrl}`);
    // TODO: pass through auth token
    watch(
      `${this.resourceUrl}?watch=1&resourceVersion=${resourceVersion}`,
      {
        Authorization: 'Bearer ' + this.auth.token,
      },
      handler
    );
  }
}
