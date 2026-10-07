import { createPinia, setActivePinia } from 'pinia';
import type { RouteLocationNormalized } from 'vue-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiService } from '@/common/services/api/api.service';
import { ApiError } from '@/common/services/api/api-error';
import { authGuard } from '../guards';

vi.mock('@/common/services/api/api.service', () => ({
  apiService: { auth: { login: vi.fn(), logout: vi.fn(), me: vi.fn() } },
}));

function routeTo(fullPath: string, publicAccess = false): RouteLocationNormalized {
  return { fullPath, meta: { publicAccess } } as unknown as RouteLocationNormalized;
}

describe('authGuard', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.mocked(apiService.auth.me).mockReset();
  });

  it('lets public routes through without asking the backend', async () => {
    expect(await authGuard(routeTo('/login', true))).toBe(true);
    expect(apiService.auth.me).not.toHaveBeenCalled();
  });

  it('sends a signed-out user to login and remembers the target', async () => {
    vi.mocked(apiService.auth.me).mockRejectedValue(new ApiError(401, 'unauthenticated', 'x'));

    expect(await authGuard(routeTo('/users'))).toEqual({ name: 'login', query: { redirect: '/users' } });
  });

  it('treats an unreachable backend as signed out', async () => {
    vi.mocked(apiService.auth.me).mockRejectedValue(new ApiError(0, 'network', 'down'));

    expect(await authGuard(routeTo('/users'))).toEqual({ name: 'login', query: { redirect: '/users' } });
  });

  it('asks the backend only once per page load', async () => {
    vi.mocked(apiService.auth.me).mockResolvedValue({ uid: 'alice', commonName: 'Alice Admin' });

    expect(await authGuard(routeTo('/'))).toBe(true);
    expect(await authGuard(routeTo('/users'))).toBe(true);
    expect(apiService.auth.me).toHaveBeenCalledOnce();
  });
});
