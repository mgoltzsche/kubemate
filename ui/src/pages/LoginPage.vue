<template>
  <q-page class="items-center row justify-evenly">
    <login-form />
  </q-page>
</template>

<script lang="ts">
import { defineComponent } from 'vue';
import LoginForm from 'src/components/LoginForm.vue';

export default defineComponent({
  name: 'LoginPage',
  components: { LoginForm },
});
</script>
