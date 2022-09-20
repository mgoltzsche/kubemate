import { useQuasar } from 'quasar';

export function error(e: any) {
  console.log('ERROR:', e);
  const quasar = useQuasar();
  quasar.notify({
    type: 'negative',
    message: e.body?.message ? `${e.message}: ${e.body?.message}` : e.message,
  });
}

export function catchError(p: Promise<any>) {
  p.catch((e: any) => {
    error(e);
  });
}
