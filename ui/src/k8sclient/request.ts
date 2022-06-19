import { CancelablePromise } from './CancelablePromise';
import type { OnCancel } from './CancelablePromise';

export type ApiRequestOptions = {
  readonly method:
    | 'GET'
    | 'PUT'
    | 'POST'
    | 'DELETE'
    | 'OPTIONS'
    | 'HEAD'
    | 'PATCH';
  readonly url: string;
  readonly headers?: Record<string, any>;
  readonly query?: Record<string, any>;
  readonly formData?: Record<string, any>;
  readonly body?: any;
  readonly responseHeader?: string;
  readonly errors?: Record<number, string>;
};

export class ApiError extends Error {
  public readonly url: string;
  public readonly status: number;
  public readonly statusText: string;
  public readonly body: any;

  constructor(response: ApiResult, message: string) {
    super(message);

    this.name = 'ApiError';
    this.url = response.url;
    this.status = response.status;
    this.statusText = response.statusText;
    this.body = response.body;
  }
}

export type ApiResult = {
  readonly url: string;
  readonly ok: boolean;
  readonly status: number;
  readonly statusText: string;
  readonly body: any;
};

function isDefined<T>(
  value: T | null | undefined
): value is Exclude<T, null | undefined> {
  return value !== undefined && value !== null;
}

function isString(value: any): value is string {
  return typeof value === 'string';
}

function isBlob(value: any): value is Blob {
  return value instanceof Blob;
}

function getQueryString(params: Record<string, any>): string {
  const qs: string[] = [];

  Object.keys(params).forEach((key) => {
    const value = params[key];
    if (isDefined(value)) {
      if (Array.isArray(value)) {
        value.forEach((value) => {
          qs.push(
            `${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`
          );
        });
      } else {
        qs.push(
          `${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`
        );
      }
    }
  });

  if (qs.length > 0) {
    return `?${qs.join('&')}`;
  }

  return '';
}

function getUrl(options: ApiRequestOptions): string {
  if (options.query) {
    return `${options.url}${getQueryString(options.query)}`;
  }
  return options.url;
}

function getFormData(options: ApiRequestOptions): FormData | undefined {
  if (options.formData) {
    const formData = new FormData();

    Object.entries(options.formData)
      .filter(([_, value]) => isDefined(value))
      .forEach(([key, value]) => {
        if (isString(value) || isBlob(value)) {
          formData.append(key, value);
        } else {
          formData.append(key, JSON.stringify(value));
        }
      });

    return formData;
  }
  return;
}

type Resolver<T> = (options: ApiRequestOptions) => Promise<T>;

async function resolve<T>(
  options: ApiRequestOptions,
  resolver?: T | Resolver<T>
): Promise<T | undefined> {
  if (typeof resolver === 'function') {
    return (resolver as Resolver<T>)(options);
  }
  return resolver;
}

async function getHeaders(options: ApiRequestOptions): Promise<Headers> {
  const defaultHeaders = Object.entries({
    Accept: 'application/json',
    ...options.headers,
  })
    .filter(([_, value]) => isDefined(value))
    .reduce(
      (headers, [key, value]) => ({
        ...headers,
        [key]: String(value),
      }),
      {} as Record<string, string>
    );

  const headers = new Headers(defaultHeaders);

  if (options.body) {
    headers.append('Content-Type', 'application/json');
  }

  return headers;
}

function getRequestBody(options: ApiRequestOptions): BodyInit | undefined {
  if (options.body) {
    return JSON.stringify(options.body);
  }
  return;
}

async function sendRequest(
  options: ApiRequestOptions,
  url: string,
  formData: FormData | undefined,
  body: BodyInit | undefined,
  headers: Headers,
  onCancel: OnCancel
): Promise<Response> {
  const controller = new AbortController();

  const request: RequestInit = {
    headers,
    body: body || formData,
    method: options.method,
    signal: controller.signal,
  };

  onCancel(() => controller.abort());

  return await fetch(url, request);
}

function getResponseHeader(
  response: Response,
  responseHeader?: string
): string | undefined {
  if (responseHeader) {
    const content = response.headers.get(responseHeader);
    if (isString(content)) {
      return content;
    }
  }
  return;
}

async function getResponseBody(response: Response): Promise<any> {
  if (response.status !== 204) {
    try {
      const contentType = response.headers.get('Content-Type');
      if (contentType) {
        const isJSON = contentType.toLowerCase().startsWith('application/json');
        if (isJSON) {
          return await response.json();
        } else {
          return await response.text();
        }
      }
    } catch (error) {
      console.error(error);
    }
  }
  return;
}

function catchErrors(options: ApiRequestOptions, result: ApiResult): void {
  const errors: Record<number, string> = {
    400: 'Bad Request',
    401: 'Unauthorized',
    403: 'Forbidden',
    404: 'Not Found',
    500: 'Internal Server Error',
    502: 'Bad Gateway',
    503: 'Service Unavailable',
    ...options.errors,
  };

  const error = errors[result.status];
  if (error) {
    throw new ApiError(result, error);
  }

  if (!result.ok) {
    throw new ApiError(result, 'Generic Error');
  }
}

/**
 * Request using fetch client
 * @param options The request options from the the service
 * @returns CancelablePromise<T>
 * @throws ApiError
 */
export function request<T>(options: ApiRequestOptions): CancelablePromise<T> {
  return new CancelablePromise(async (resolve, reject, onCancel) => {
    try {
      const url = getUrl(options);
      const formData = getFormData(options);
      const body = getRequestBody(options);
      const headers = await getHeaders(options);

      if (!onCancel.isCancelled) {
        const response = await sendRequest(
          options,
          url,
          formData,
          body,
          headers,
          onCancel
        );
        const responseBody = await getResponseBody(response);
        const responseHeader = getResponseHeader(
          response,
          options.responseHeader
        );

        const result: ApiResult = {
          url,
          ok: response.ok,
          status: response.status,
          statusText: response.statusText,
          body: responseHeader || responseBody,
        };

        catchErrors(options, result);

        resolve(result.body);
      }
    } catch (error) {
      reject(error);
    }
  });
}
