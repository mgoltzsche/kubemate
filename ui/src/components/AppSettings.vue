<template>
  <div v-if="!ready">
    <q-skeleton type="rect" />
  </div>
  <div v-if="ready">
    <q-card flat>
      <q-card-section>
        <div class="text-h6">{{ app.metadata?.name }} settings</div>
      </q-card-section>

      <q-card-section class="q-pt-none q-gutter-y-md">
        <div>
          <div class="col-auto q-pa-md" v-if="ready">
            <q-tabs
              v-model="selectedCategory"
              dense
              narrow-indicator
              inline-label
              class="text-primary"
              active-color="primary"
              indicator-color="primary"
              align="left"
            >
              <q-tab
                :name="category"
                :label="category"
                v-for="category in categories"
                v-bind:key="category"
              />
            </q-tabs>
            <q-separator />
          </div>
          <q-tab-panels v-model="selectedCategory" animated swipeable>
            <q-tab-panel
              :name="category"
              v-for="category in categories"
              v-bind:key="category"
            >
              <div class="q-pa-md">
                <div class="q-gutter-md">
                  <div v-for="param in params" v-bind:key="param.name">
                    <multi-line-input
                      filled
                      :autogrow="inputType(param.type) == 'textarea'"
                      v-model="secretData[param.name]"
                      :default-value="defaultData[param.name] || ''"
                      :label="
                        (param.title || param.name) +
                        (defaultData[param.name] ? '' : '*')
                      "
                      :label-color="
                        defaultData[param.name] || secretData[param.name]
                          ? 'text'
                          : 'negative'
                      "
                      :type="inputType(param.type)"
                      :rules="[
                        (v: unknown) =>
                          !!defaultData[param.name] ||
                          !!v ||
                          'Field is required',
                      ]"
                      v-if="param.type != 'boolean' && param.type != 'enum'"
                    >
                      <template v-slot:after v-if="param.description">
                        <q-btn
                          round
                          dense
                          flat
                          icon="help_outline"
                          @click="showHelpDialog(param)"
                        />
                      </template>
                    </multi-line-input>
                    <q-toggle
                      v-model="boolData[param.name]"
                      :label="param.title || param.name"
                      :hint="param.description"
                      v-if="param.type == 'boolean'"
                    />
                    <q-select
                      v-model="secretData[param.name]"
                      :label="
                        (param.title || param.name) +
                        (defaultData[param.name] ? '' : '*')
                      "
                      :options="param.enum"
                      filled
                      v-if="param.type == 'enum'"
                    />
                  </div>
                </div>
              </div>
            </q-tab-panel>
          </q-tab-panels>
        </div>
      </q-card-section>
      <q-card-actions>
        <q-btn color="primary" label="Apply" @click="apply" />
        <q-toggle
          v-model="showOptionalParams"
          label="Show all"
          @click="selectFirstVisibleTab"
        />
      </q-card-actions>
    </q-card>

    <q-dialog v-model="helpDialogOpen">
      <q-card>
        <q-card-section class="row items-center q-pb-none">
          <div class="text-h6">
            <q-icon name="help_outline" />
            {{ helpHeadline }}
          </div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>
        <q-card-section>
          {{ helpText }}
        </q-card-section>
      </q-card>
    </q-dialog>
  </div>
</template>

<script lang="ts">
import { computed, defineComponent, reactive, toRefs, ref } from 'vue';
import apiclient from 'src/k8sclient';
import { info, catchError } from 'src/notify';
import {
  com_github_mgoltzsche_kubemate_pkg_apis_apps_v1alpha1_App as App,
  com_github_mgoltzsche_kubemate_pkg_apis_apps_v1alpha1_ParameterDefinition as ParameterDefinition,
  io_k8s_api_core_v1_Secret as Secret,
} from 'src/gen';
import { useAppStore } from 'src/stores/resources';
import { Notify } from 'quasar';
import { com_github_mgoltzsche_kubemate_pkg_apis_apps_v1alpha1_AppConfigSchema as AppConfigSchema } from 'src/gen';
import MultiLineInput from 'src/components/MultiLineInput.vue';

const kc = new apiclient.KubeConfig();
const secretsClient = kc.newClient<Secret>('/api/v1', 'secrets');
const configSchemaClient = kc.newClient<AppConfigSchema>(
  '/apis/apps.kubemate.mgoltzsche.github.com/v1alpha1',
  'appconfigschemas'
);
const defaultCategory = 'General';

function categoryName(p: ParameterDefinition) {
  return p.category || defaultCategory;
}

function secretName(a: App) {
  return `${a.metadata?.name}-userconfig`;
}

function inputType(
  s: ParameterDefinition.type | undefined
): 'text' | 'textarea' | 'password' | 'number' {
  switch (s) {
    case ParameterDefinition.type.TEXT:
      return 'textarea';
    case ParameterDefinition.type.PASSWORD:
      return 'password';
    case ParameterDefinition.type.NUMBER:
      return 'number';
    case ParameterDefinition.type.STRING:
    default:
      return 'text';
  }
}

export default defineComponent({
  name: 'AppSettings',
  components: { MultiLineInput },
  props: {
    app: {
      type: String,
      required: true,
    },
  },
  setup(props) {
    const ready = ref(false);
    const app = ref<App>({ spec: {} });
    const configSchema = ref<AppConfigSchema>({ spec: {} });
    const defaultData = ref<Record<string, string>>({});
    const secretData = ref<Record<string, string>>({});
    const boolData = ref<Record<string, boolean>>({});
    const selectedCategory = ref(defaultCategory);
    const showOptionalParams = ref(false);
    const helpDialogOpen = ref(false);
    const helpHeadline = ref('');
    const helpText = ref('');
    const apps = useAppStore();

    function categories() {
      return new Set(
        configSchema.value.spec.params
          ?.filter(
            (p) =>
              showOptionalParams.value ||
              !defaultData.value ||
              !defaultData.value[p.name]
          )
          ?.map(categoryName)
      );
    }

    function selectFirstVisibleTab() {
      const c = categories();
      if (c.size > 0 && !c.has(selectedCategory.value)) {
        selectedCategory.value = c.values().next().value;
      }
    }

    function base64Decode(d?: Record<string, string>) {
      const r = {} as Record<string, string>;
      for (const k in d) {
        r[k] = atob(d[k]);
      }
      return r;
    }

    function base64Encode(d: Record<string, string>) {
      const r = {} as Record<string, string>;
      for (const k in d) {
        r[k] = btoa(d[k]);
      }
      return r;
    }

    function populateBoolData() {
      boolData.value =
        configSchema.value.spec.params
          ?.filter((p) => p.type === ParameterDefinition.type.BOOLEAN)
          .reduce<Record<string, boolean>>((r, p) => {
            r[p.name] = secretData.value[p.name] === 'true';
            return r;
          }, {}) || {};
    }

    apps.sync(() => {
      const a = apps.resources.find((a) => a.metadata?.name == props.app);
      if (!a) {
        Notify.create({
          type: 'negative',
          message: `App ${props.app} does not exist!`,
        });
        return;
      }
      const csName = a.status?.configSchemaName;
      if (!csName) {
        Notify.create({
          type: 'negative',
          message: `${props.app} app does not specify a configuration schema!`,
        });
        return;
      }
      catchError(
        configSchemaClient.get(csName, a.metadata?.namespace).then((cs) => {
          configSchema.value = cs;
          app.value = a;

          return secretsClient
            .get(`${a.metadata?.name}-defaultconfig`, a.metadata?.namespace)
            .then((s) => {
              defaultData.value = base64Decode(s.data);

              secretsClient
                .get(secretName(a), a.metadata?.namespace)
                .then((s) => {
                  secretData.value = base64Decode(s.data);
                })
                .catch((e) => {
                  secretData.value = {};
                })
                .finally(() => {
                  cs.spec.params?.forEach((p) => {
                    if (!(p.name in secretData.value)) {
                      secretData.value[p.name] =
                        defaultData.value[p.name] || '';
                    }
                  });
                  populateBoolData();
                  ready.value = true;
                  selectFirstVisibleTab();
                });
            });
        })
      );
    });
    const state = reactive({
      ready: ready,
      app: app,
      secretData: secretData,
      defaultData: defaultData,
      boolData: boolData,
      selectedCategory: selectedCategory,
      showOptionalParams: showOptionalParams,
      helpDialogOpen: helpDialogOpen,
      helpHeadline: helpHeadline,
      helpText: helpText,
      categories: computed(categories),
      params: computed(() => {
        return configSchema.value.spec.params?.filter(
          (p) =>
            categoryName(p) == selectedCategory.value &&
            (showOptionalParams.value ||
              !defaultData.value ||
              !defaultData.value[p.name])
        );
      }),
      showHelpDialog: (p: ParameterDefinition) => {
        helpDialogOpen.value = true;
        helpHeadline.value = p.title || p.name;
        helpText.value = p.description || '';
      },
      apply: () => {
        secretsClient
          .get(secretName(app.value), app.value.metadata?.namespace)
          .then((s) => {
            // Update application settings Secret resource
            if (!configSchema.value.spec.params) return;
            configSchema.value.spec.params
              .filter((p) => p.type === ParameterDefinition.type.BOOLEAN)
              .reduce<Record<string, string>>((r, p) => {
                r[p.name] = boolData.value[p.name] ? 'true' : 'false';
                return r;
              }, secretData.value);
            s.data = base64Encode(
              configSchema.value.spec.params.reduce<Record<string, string>>(
                (d, p) => {
                  if (secretData.value[p.name] === defaultData.value[p.name]) {
                    delete d[p.name];
                  }
                  return d;
                },
                Object.assign({}, secretData.value)
              )
            );
            catchError(
              secretsClient
                .update(s)
                .then(() => info('app configuration saved'))
            );
          })
          .catch(() => {
            // Create new Secret
            const s = {
              metadata: {
                name: secretName(app.value),
                namespace: app.value.metadata?.namespace,
              },
              data: base64Encode(secretData.value),
            };
            catchError(
              secretsClient
                .create(s)
                .then(() => info('app configuration saved'))
            );
          });
      },
    });
    return {
      ...toRefs(state),
      inputType,
      selectFirstVisibleTab,
    };
  },
});
</script>
