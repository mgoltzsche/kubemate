import { defineStore } from 'pinia';

const localStorageTokenKey = 'login-token';

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: window.localStorage.getItem(localStorageTokenKey) || '',
  }),
  getters: {
    authenticated: (state) => state.token.length > 0,
  },
  actions: {
    setToken(token: string) {
      this.token = token;
      window.localStorage.setItem(localStorageTokenKey, token);
    },
  },
});
