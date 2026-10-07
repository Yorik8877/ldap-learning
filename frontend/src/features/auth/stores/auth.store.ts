import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { apiService } from '@/common/services/api/api.service';
import { isApiError } from '@/common/services/api/api-error';
import type { CurrentUser } from '@/common/services/api/models';

export const useAuthStore = defineStore('auth', () => {
  const user = ref<CurrentUser | null>(null);
  // checked — сессию уже спрашивали у бэкенда; без этого guard ходил бы в /auth/me на каждый переход.
  const checked = ref(false);
  const isSignedIn = computed(() => user.value !== null);

  async function fetchCurrentUser(): Promise<void> {
    try {
      user.value = await apiService.auth.me();
    } catch (caught) {
      user.value = null;
      if (!isApiError(caught) || caught.code !== 'unauthenticated') {
        throw caught;
      }
    } finally {
      checked.value = true;
    }
  }

  async function signIn(uid: string, password: string): Promise<void> {
    user.value = await apiService.auth.login(uid, password);
    checked.value = true;
  }

  async function signOut(): Promise<void> {
    try {
      await apiService.auth.logout();
    } finally {
      reset();
    }
  }

  function reset(): void {
    user.value = null;
  }

  return { user, checked, isSignedIn, fetchCurrentUser, signIn, signOut, reset };
});
