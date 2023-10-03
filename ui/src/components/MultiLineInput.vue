<template>
  <q-input
    v-model="value"
    @paste="handlePaste"
    @update:model-value="handleInput"
  >
    <template v-slot:append>
      <q-icon
        v-if="value"
        name="close"
        @click="clear"
        class="cursor-pointer"
      />
    </template>
    <template v-slot:after>
      <slot name="after"></slot>
    </template>
  </q-input>
</template>

<script lang="ts">
import { defineComponent } from 'vue';

export default defineComponent({
  name: 'MultiLineInput',
  props: {
    modelValue: {
      type: String,
      default: '',
    },
    defaultValue: {
      type: String,
      default: '',
    },
  },
  data() {
    return {
      value: this.modelValue,
    };
  },
  methods: {
    handleInput() {
      this.$emit('update:modelValue', this.value);
    },
    handlePaste(evt: ClipboardEvent) {
      evt.preventDefault();

      const clipboardData = evt.clipboardData;
      if (clipboardData) {
        const v = clipboardData.getData('text');
        this.value = v;
        this.$emit('update:modelValue', v);
      }
    },
    clear() {
      this.value = this.defaultValue || '';
      this.$emit('update:modelValue', this.value);
    }
  },
});
</script>
