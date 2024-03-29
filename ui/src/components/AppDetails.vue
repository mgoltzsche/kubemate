<template>
  <div v-if="!app && synchronizing">
    <q-skeleton type="rect" />
  </div>
  <div v-if="!app && !synchronizing">App not found</div>
  <div v-if="app">
    <q-card flat class="my-card">
      <q-card-section>
        <div class="text-h6">{{ app.metadata?.name }}</div>
      </q-card-section>

      <q-card-section class="q-pt-none">
        <q-markup-table separator="none" flat>
          <tbody>
            <tr>
              <th class="text-left">State</th>
              <td class="text-left">
                <span>{{ app.status?.state }}</span>
                <span v-if="app.status?.message"
                  >: {{ app.status?.message }}</span
                >
              </td>
            </tr>
            <tr v-if="app.status?.lastAppliedRevision">
              <th class="text-left">Applied revision</th>
              <td class="text-left">{{ app.status?.lastAppliedRevision }}</td>
            </tr>
            <tr
              v-if="
                app.status?.lastAttemptedRevision &&
                app.status?.lastAttemptedRevision !=
                  app.status.lastAppliedRevision
              "
            >
              <th class="text-left">Attempting revision</th>
              <td class="text-left">{{ app.status?.lastAttemptedRevision }}</td>
            </tr>
          </tbody>
        </q-markup-table>
      </q-card-section>

      <q-card-section>
        <app-launcher :app="appName" />
      </q-card-section>

      <q-card-actions>
        <q-btn
          clickable
          :label="installButtonLabel"
          color="secondary"
          @click="installOrUninstallApp"
        />
        <q-btn
          clickable
          label="Configure"
          color="primary"
          :href="`#/apps/${appName}/settings`"
          v-if="app.status?.configSchemaName"
        />
      </q-card-actions>
    </q-card>
  </div>
</template>

<script lang="ts">
import { computed, defineComponent, reactive, toRefs } from 'vue';
import { useAppStore } from 'src/stores/resources';
import AppLauncher from 'components/AppLauncher.vue';
import { catchError } from 'src/notify';

export default defineComponent({
  name: 'AppDetails',
  components: {
    AppLauncher,
  },
  props: {
    appName: {
      type: String,
      required: true,
    },
  },
  setup(props) {
    const store = useAppStore();
    store.sync();

    const state = reactive({
      synchronizing: store.synchronizing,
      appName: props.appName,
      app: computed(() =>
        store.resources.find((a) => a.metadata?.name == props.appName)
      ),
      installButtonLabel: computed(() => {
        const app = store.resources.find(
          (a) => a.metadata?.name === props.appName
        );
        return app?.spec?.enabled ? 'uninstall' : 'install';
      }),
      installOrUninstallApp: () => {
        const app = store.resources.find(
          (a) => a.metadata?.name === props.appName
        );
        if (!app) return;
        app.spec.enabled = !app.spec.enabled;
        // TODO: support namespaced resources
        catchError(store.client.update(app));
      },
    });
    return { ...toRefs(state) };
  },
});
</script>
