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
  handler: (evt: WatchEvent<T>) => void
): void {
  streamFromURL<WatchEvent<T>>(url)
    .then((stream) => {
      return consumeStream(stream, (evt) => {
        console.log('watch: received event:', evt);
        handler(evt);
      });
    })
    .catch((e) => {
      console.log('ERROR: watch failed:', new Date(), e);
      console.log('watch: restarting...');
      setTimeout(() => {
        watch(url, handler);
      }, 1000); // retry after 1s
    });
}

async function consumeStream<T>(
  stream: ReadableStream<T>,
  consume: (value: T) => void
) {
  const reader = stream.getReader();
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

function streamFromURL<T>(url: string): Promise<ReadableStream<T>> {
  return fetch(url).then((response) => {
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
        // TODO: check if any of this fails and reorder and/or catch it
        reader.cancel();
        response.body?.cancel();
      },
    });
    return stream;
  });
}
