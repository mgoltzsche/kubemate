<template>
  <cli-login />
</template>

<script lang="ts">
import { defineComponent } from 'vue';
import CliLogin from 'src/components/CLILogin.vue';

export default defineComponent({
  name: 'CLILoginPage',
  components: { CliLogin },
});
</script>
