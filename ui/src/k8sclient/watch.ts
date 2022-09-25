import { CancelablePromise } from './CancelablePromise';

export type WatchEvent<T> = {
  readonly type: EventType;
  readonly object: T & Error;
};

export type Error = {
  readonly message: string;
};

export enum EventType {
  ADDED = 'ADDED',
  MODIFIED = 'MODIFIED',
  DELETED = 'DELETED',
  BOOKMARK = 'BOOKMARK',
  ERROR = 'ERROR',
}

export function watch<T>(
  url: string,
  headers: Record<string, string>,
  handler: (evt: WatchEvent<T>) => void
): CancelablePromise<void> {
  return new CancelablePromise(async (resolve, reject, onCancel) => {
    try {
      const stream = streamFromURL<WatchEvent<T>>(url, headers);
      onCancel(async () => {
        await stream.cancel();
      });
      const reader = (await stream).getReader();
      onCancel(async () => {
        await reader.cancel();
        await stream.cancel();
      });
      await consumeStream(reader, (evt) => {
        console.log('client: watch: received event', evt);
        handler(evt);
      });
      resolve();
    } catch (e) {
      reject(e);
    }
  });
}

async function consumeStream<T>(
  reader: ReadableStreamReader<T>,
  consume: (value: T) => void
) {
  let value: T | undefined;
  let done = false;
  while (!done) {
    ({ value, done } = await reader.read());
    if (done) {
      console.log('ERROR: watch: server terminated stream');
      return;
    }
    if (value) consume(value);
  }
}

function streamFromURL<T>(
  url: string,
  headers: Record<string, string>
): CancelablePromise<ReadableStream<T>> {
  return new CancelablePromise(async (resolve, reject, onCancel) => {
    try {
      const abortController = new AbortController();
      const signal = abortController.signal;
      const streamPromise = fetch(url, {
        headers: headers,
        signal: signal,
      }).then((response) => {
        if (response.status != 200) {
          return Promise.reject(
            `request ${url}: server responded with status code ${response.status}`
          );
        }
        if (!response.body) {
          return Promise.reject(`request ${url}: missing response body`);
        }
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let text = '';
        const stream = new ReadableStream<T>({
          start(controller: ReadableStreamController<T>) {
            function push(): Promise<void> {
              return reader
                .read()
                .then(({ done, value }) => {
                  if (done) {
                    controller.close();
                    return;
                  }
                  text += decoder.decode(value || new Uint8Array());
                  for (
                    let pos = text.indexOf('\n');
                    pos > -1;
                    pos = text.indexOf('\n')
                  ) {
                    const line = text.substring(0, pos).trim();
                    if (line) {
                      controller.enqueue(JSON.parse(line));
                    }
                    text = text.substring(pos + 1);
                  }
                })
                .then(push)
                .catch((e) => {
                  controller.error(e);
                });
            }
            push();
          },
          cancel() {
            abortController.abort();
          },
        });
        return stream;
      });
      onCancel(() => abortController.abort());
      resolve(await streamPromise);
    } catch (e) {
      reject(e);
    }
  });
}
