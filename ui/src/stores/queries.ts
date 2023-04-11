import { io_k8s_api_networking_v1_Ingress as Ingress } from 'src/gen';

const navTitleAnnotation = 'kubemate.mgoltzsche.github.com/nav-title';

export function appLinks(ingresses: Ingress[]) {
  return ingresses
    .filter(
      (ing: Ingress) =>
        ing.metadata?.annotations &&
        ing.metadata?.annotations[navTitleAnnotation]
    )
    .map((ing: Ingress) => {
      const a = ing.metadata?.annotations;
      const title = a ? a[navTitleAnnotation] : '';
      const key = `${ing.metadata?.namespace}/${ing.metadata?.name}`;
      const url = ing.spec?.rules?.find((r) => {
        return !r.host && r.http?.paths && r.http.paths.length > 0;
      })?.http?.paths[0].path;
      return {
        key: key,
        title: title ? title : key,
        caption: title ? key : '',
        info: title ? `${title} (${key})` : key,
        url: url,
        iconPath: a ? a['kubemate.mgoltzsche.github.com/nav-icon'] : '',
      };
    })
    .filter((e) => e.url);
}
