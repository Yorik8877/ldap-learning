import type { RouteLocationNormalized, RouteLocationRaw } from 'vue-router';
import { useAuthStore } from '@/features/auth';

export async function authGuard(to: RouteLocationNormalized): Promise<true | RouteLocationRaw> {
  if (to.meta.publicAccess) {
    return true;
  }
  const auth = useAuthStore();
  if (!auth.checked) {
    try {
      await auth.fetchCurrentUser();
    } catch {
      auth.reset();
    }
  }
  if (auth.isSignedIn) {
    return true;
  }
  return { name: 'login', query: { redirect: to.fullPath } };
}
