import { defineStore } from 'pinia';

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: '',
  }),
  getters: {
    authenticated: (state) => state.token.length > 0,
  },
  actions: {
    setToken(token: string) {
      this.token = token;
    },
  },
});
