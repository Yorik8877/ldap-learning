import type { AxiosInstance } from 'axios';
import { describe, expect, it, vi } from 'vitest';
import { AuthRepository } from '../repositories/auth.repository';

describe('AuthRepository', () => {
  it('maps the backend user to the model', async () => {
    const post = vi.fn().mockResolvedValue({ data: { uid: 'alice', cn: 'Alice Admin' } });
    const repository = new AuthRepository({ post } as unknown as AxiosInstance);

    const user = await repository.login('alice', 'alice-secret');

    expect(post).toHaveBeenCalledWith('/auth/login', { uid: 'alice', password: 'alice-secret' });
    expect(user).toEqual({ uid: 'alice', commonName: 'Alice Admin' });
  });
});
