<template>
  <q-card
    square
    class="shadow-24"
    style="width: 400px; height: 140px"
    v-if="authenticated"
  >
    <q-card-section class="text-center">
      You are logged in on device {{ deviceName }}!
    </q-card-section>
    <q-card-actions class="q-px-lg">
      <q-btn
        unelevated
        size="lg"
        color="secondary"
        @click="logout"
        class="full-width text-white"
        label="Logout"
      />
    </q-card-actions>
  </q-card>

  <q-card
    square
    class="shadow-24"
    style="width: 400px; height: 370px"
    v-if="!authenticated"
  >
    <q-card-section class="bg-blue-grey-9">
      <h2 class="text-h5 text-white q-my-md">Login</h2>
      <div class="text-subtitle2 text-white">@{{ deviceName }}</div>
    </q-card-section>

    <q-card-section>
      <q-form class="q-px-sm q-pt-xl" @submit="login">
        <q-input
          square
          clearable
          v-model="token"
          :type="passwordFieldType"
          lazy-rules
          :rules="[required]"
          label="Token"
        >
          <template v-slot:prepend>
            <q-icon name="lock" />
          </template>
          <template v-slot:append>
            <q-icon
              :name="visibilityIcon"
              @click="switchVisibility"
              class="cursor-pointer"
            />
          </template>
        </q-input>
      </q-form>
    </q-card-section>
    <q-card-actions class="q-px-lg">
      <q-btn
        unelevated
        size="lg"
        color="primary"
        @click="login"
        class="full-width text-white"
        label="Login"
      />
    </q-card-actions>
  </q-card>
</template>

<script lang="ts">
import { defineComponent, ref, toRefs, reactive, computed } from 'vue';
import { useAuthStore } from 'src/stores/auth';
import { useDeviceStore } from 'src/stores/resources';

function useTokenVisibilityToggle() {
  const visible = ref(false);
  const passwordFieldType = ref('password');
  const visibilityIcon = ref('visibility_off');
  function switchVisibility() {
    visible.value = !visible.value;
    passwordFieldType.value = visible.value ? 'text' : 'password';
    visibilityIcon.value = visible.value ? 'visibility' : 'visibility_off';
  }
  return { passwordFieldType, visibilityIcon, switchVisibility };
}

function useAuthentication() {
  const auth = useAuthStore();
  const token = ref('');
  function login() {
    auth.setToken(token.value);
    token.value = '';
  }
  function logout() {
    auth.setToken('');
  }
  function required(val: string) {
    return (val && val.length > 0) || 'Required!';
  }
  const state = {
    authenticated: computed(() => auth.token && auth.token.length > 0),
  };
  return { token, login, logout, required, ...toRefs(state) };
}

function useDeviceInfo() {
  const store = useDeviceStore();
  store.sync();
  const state = reactive({
    deviceName: computed(
      () => store.resources.find((d) => d.status.current)?.metadata.name || ''
    ),
  });
  return {
    ...toRefs(state),
  };
}

export default defineComponent({
  name: 'LoginForm',
  setup() {
    return {
      ...useTokenVisibilityToggle(),
      ...useAuthentication(),
      ...useDeviceInfo(),
    };
  },
});
</script>
