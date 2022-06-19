/*
import k8s from '@kubernetes/client-node';

// Fails due to incomplete polyfills:
// Requires polyfills for os etc but node-polyfill-webpack-plugin is not sufficient since it doesn support os.constants!
// See https://github.com/webpack/node-libs-browser/issues/78
// Apparently the os package is used here or rather by ExecAuth.
// See https://github.com/kubernetes-client/javascript/blob/62ef490ef2e5eb3393b6676f8e5767749c51f574/src/config.ts#L42
// and https://github.com/kubernetes-client/javascript/issues/165
const kc = new k8s.KubeConfig();

const cluster = {
  name: 'my-server',
  server: 'http://server.com',
};

const user = {
  name: 'my-user',
  password: 'some-password',
};

const context = {
  name: 'my-context',
  user: user.name,
  cluster: cluster.name,
};

const kco = {
  cluster: [cluster],
  user: [user],
  context: [context],
};

export const client = kc.makeApiClient(k8s.CustomObjectsApi);
*/

/*
// The required polyfills as quasar.config.js snippet:
    build: {
      // Make kubernetes-node work within the browser
      /*chainWebpack(chain) {
        const nodePolyfillWebpackPlugin = require('node-polyfill-webpack-plugin');
        chain.plugin('node-polyfill').use(nodePolyfillWebpackPlugin);
      },
      extendWebpack(cfg) {
        cfg.resolve.fallback = {
          fs: false,
          net: false,
          tls: false,
          child_process: false,
          dns: false,
          http2: false,
          os: './src/os-browserify-fix', // doesn't replace unfortunately
        };
      },
      ...
    }
*/
