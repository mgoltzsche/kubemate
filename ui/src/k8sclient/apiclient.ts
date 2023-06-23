import { request } from './request';
import { CancelablePromise } from './CancelablePromise';
import { WatchEvent, watch } from './watch';
import { Resource, ResourceList } from './model';
import { useAuthStore } from 'src/stores/auth';

export class KubeConfig {
  newClient<T extends Resource>(
    baseUrl: string,
    resource: string
  ): ApiClient<T> {
    return new ApiClient<T>(baseUrl, resource);
  }
}

export class ApiClient<T extends Resource> {
  private auth = useAuthStore();
  private baseUrl: string;
  private res: string;
  constructor(baseUrl: string, resource: string) {
    this.baseUrl = baseUrl;
    this.res = resource;
  }
  private resourceUrl(namespace?: string): string {
    return namespace
      ? `${this.baseUrl}/namespaces/${namespace}/${this.res}`
      : `${this.baseUrl}/${this.res}`;
  }
  public resource(): string {
    return this.res;
  }
  public create(o: T): CancelablePromise<T> {
    const url = this.resourceUrl(o.metadata?.namespace);
    console.log(`client: create ${url}`);
    return request({
      method: 'POST',
      url: url,
      query: {
        timeoutSeconds: 10,
      },
      headers: {
        Authorization: 'Bearer ' + this.auth.token,
      },
      body: o,
    });
  }
  public update(o: T): CancelablePromise<T> {
    const name = o.metadata?.name;
    const url = `${this.resourceUrl(o.metadata?.namespace)}/${name}`;
    console.log(`client: update ${url}`);
    return request({
      method: 'PUT',
      url: url,
      query: {
        timeoutSeconds: 10,
      },
      headers: {
        Authorization: 'Bearer ' + this.auth.token,
      },
      body: o,
    });
  }
  public delete(
    name: string,
    namespace?: string
  ): CancelablePromise<ResourceList<T>> {
    const url = `${this.resourceUrl(namespace)}/${name}`;
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
    const url = `${this.resourceUrl(namespace)}/${name}`;
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
    const url = this.resourceUrl();
    console.log(`client: list ${url}`);
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
  public watch(
    handler: (evt: WatchEvent<T>) => void,
    resourceVersion?: string
  ): CancelablePromise<void> {
    const url = this.resourceUrl();
    console.log(`client: watch ${url}`);
    return watch(
      `${url}?watch=1&resourceVersion=${resourceVersion}`,
      {
        Authorization: 'Bearer ' + this.auth.token,
      },
      handler
    );
  }
  public createSubresource<S>(
    o: T,
    subResource: string,
    s: S
  ): CancelablePromise<T> {
    const url = `${this.resourceUrl(o.metadata?.namespace)}/${
      o.metadata?.name
    }/${subResource}`;
    console.log(`client: create subresource ${url}`);
    return request({
      method: 'POST',
      url: url,
      query: {
        timeoutSeconds: 10,
      },
      headers: {
        Authorization: 'Bearer ' + this.auth.token,
      },
      body: s,
    });
  }
}
