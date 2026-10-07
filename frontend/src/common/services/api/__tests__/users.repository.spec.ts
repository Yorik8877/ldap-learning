import type { AxiosInstance } from 'axios';
import { describe, expect, it, vi } from 'vitest';
import { UsersRepository } from '../repositories/users.repository';

const johnDto = { uid: 'jdoe', cn: 'Doe, John', sn: 'Doe', mail: ['jdoe@example.com'] };
const john = { uid: 'jdoe', commonName: 'Doe, John', surname: 'Doe', emails: ['jdoe@example.com'] };

describe('UsersRepository', () => {
  it('maps users and details', async () => {
    const get = vi.fn()
      .mockResolvedValueOnce({ data: [johnDto] })
      .mockResolvedValueOnce({ data: { ...johnDto, groups: ['team'] } });
    const repository = new UsersRepository({ get } as unknown as AxiosInstance);

    expect(await repository.list()).toEqual([john]);
    expect(await repository.get('jdoe')).toEqual({ ...john, groups: ['team'] });
    expect(get).toHaveBeenLastCalledWith('/users/jdoe');
  });

  it('sends LDAP attribute names to the backend', async () => {
    const post = vi.fn().mockResolvedValue({ data: johnDto });
    const put = vi.fn().mockResolvedValue({ data: johnDto });
    const repository = new UsersRepository({ post, put } as unknown as AxiosInstance);

    await repository.create({ uid: 'jdoe', commonName: 'Doe, John', surname: 'Doe', emails: [], password: 'correct-horse' });
    await repository.update('jdoe', { commonName: 'John', surname: 'Doe', emails: ['j@example.com'] });

    expect(post).toHaveBeenCalledWith('/users', { uid: 'jdoe', cn: 'Doe, John', sn: 'Doe', mail: [], password: 'correct-horse' });
    expect(put).toHaveBeenCalledWith('/users/jdoe', { cn: 'John', sn: 'Doe', mail: ['j@example.com'] });
  });
});
