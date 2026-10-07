import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiService } from '@/common/services/api/api.service';
import { ApiError } from '@/common/services/api/api-error';
import { useAuthStore } from '../stores/auth.store';

vi.mock('@/common/services/api/api.service', () => ({
  apiService: { auth: { login: vi.fn(), logout: vi.fn(), me: vi.fn() } },
}));

describe('useAuthStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.mocked(apiService.auth.me).mockReset();
    vi.mocked(apiService.auth.logout).mockReset();
  });

  it('treats an expired session as signed out', async () => {
    vi.mocked(apiService.auth.me).mockRejectedValue(new ApiError(401, 'unauthenticated', 'x'));
    const store = useAuthStore();

    await store.fetchCurrentUser();

    expect(store.isSignedIn).toBe(false);
    expect(store.checked).toBe(true);
  });

  it('rethrows other failures', async () => {
    vi.mocked(apiService.auth.me).mockRejectedValue(new ApiError(0, 'network', 'down'));
    const store = useAuthStore();

    await expect(store.fetchCurrentUser()).rejects.toBeInstanceOf(ApiError);
    expect(store.checked).toBe(true);
  });

  it('forgets the user on sign out even if the request fails', async () => {
    vi.mocked(apiService.auth.me).mockResolvedValue({ uid: 'alice', commonName: 'Alice Admin' });
    vi.mocked(apiService.auth.logout).mockRejectedValue(new ApiError(0, 'network', 'down'));
    const store = useAuthStore();
    await store.fetchCurrentUser();

    await expect(store.signOut()).rejects.toBeInstanceOf(ApiError);

    expect(store.isSignedIn).toBe(false);
  });
});
