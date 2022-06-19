<template>
  <div v-if="!device && synchronizing">Loading...</div>
  <div v-if="!device && !synchronizing">Device not found</div>
  <div v-if="device">
    <q-card flat class="my-card">
      <q-card-section>
        <div class="text-h6">{{ device.metadata.name }}</div>
      </q-card-section>

      <q-card-section class="q-pt-none">
        Status: {{ device.status.state }} {{ device.spec.mode }}
      </q-card-section>

      <q-card-section class="q-pt-none" v-if="device.status.message">
        {{ device.status.message }}
      </q-card-section>

      <q-card-section class="q-pt-none q-gutter-y-md" style="max-width: 350px">
        <q-separator inset />
        <div>
          <q-btn-toggle
            v-model="device.spec.mode"
            :options="[
              { label: 'Server', value: 'server' },
              { label: 'Agent', value: 'agent' },
            ]"
          />

          <q-tab-panels
            v-model="device.spec.mode"
            animated
            class="shadow-2 rounded-borders"
          >
            <q-tab-panel name="server">
              The device should act as a server.
            </q-tab-panel>
            <q-tab-panel name="agent">
              <div>The device should join a server:</div>
              <q-card-section>
                <q-select
                  filled
                  clearable
                  bottom-slots
                  v-model="device.spec.server"
                  :options="availableDevices"
                  label="server"
                >
                  <template v-slot:hint
                    >The selected server manages all data and controls this
                    device.</template
                  >
                </q-select>
              </q-card-section>
            </q-tab-panel>
          </q-tab-panels>
        </div>
        <q-btn color="primary" label="Apply" />
      </q-card-section>
    </q-card>
  </div>
</template>

<script lang="ts">
import { computed, defineComponent, reactive, ref, Ref, toRefs } from 'vue';
import { useDeviceStore } from 'src/stores/resource-store';

export default defineComponent({
  name: 'DeviceDetails',
  props: {
    deviceName: {
      type: String,
      required: true,
    },
  },
  setup(props) {
    const store = useDeviceStore();
    store.sync();
    const state = reactive({
      synchronizing: store.synchronizing,
      device: computed(() =>
        store.resources.find((d) => d.metadata.name == props.deviceName)
      ),
      availableDevices: computed(() =>
        store.resources
          //.filter((d) => d.metadata.name != props.deviceName)
          .map((d) => ({ label: d.metadata.name, value: d }))
      ),
    });
    return { ...toRefs(state) };
  },
});
</script>
