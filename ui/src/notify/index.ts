import { Notify } from 'quasar';

export function error(e: any) {
  console.log('ERROR:', e);
  Notify.create({
    type: 'negative',
    message: e.body?.message
      ? `${e.message}: ${e.body?.message}`
      : e.message
      ? e.message
      : e,
  });
}

export function catchError(p: Promise<any>) {
  p.catch((e: any) => {
    error(e);
  });
}
