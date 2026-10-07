import type { AxiosInstance } from 'axios';
import { describe, expect, it, vi } from 'vitest';
import { DirectoryRepository } from '../repositories/directory.repository';

describe('DirectoryRepository', () => {
  it('omits dn for the root and maps nodes', async () => {
    const get = vi.fn().mockResolvedValue({
      data: [{ dn: 'ou=people,dc=example,dc=com', rdn: 'ou=people', objectClass: ['organizationalUnit'], hasChildren: true }],
    });
    const repository = new DirectoryRepository({ get } as unknown as AxiosInstance);

    const nodes = await repository.children(null);

    expect(get).toHaveBeenCalledWith('/directory/children', { params: {} });
    expect(nodes).toEqual([
      { dn: 'ou=people,dc=example,dc=com', rdn: 'ou=people', objectClasses: ['organizationalUnit'], hasChildren: true },
    ]);
  });

  it('passes dn as a query parameter', async () => {
    const get = vi.fn().mockResolvedValue({ data: { dn: 'x', attributes: {}, operationalAttributes: {} } });
    const repository = new DirectoryRepository({ get } as unknown as AxiosInstance);

    await repository.entry('uid=alice,ou=people,dc=example,dc=com');

    expect(get).toHaveBeenCalledWith('/directory/entry', { params: { dn: 'uid=alice,ou=people,dc=example,dc=com' } });
  });
});
