import type { AxiosInstance } from 'axios';
import { describe, expect, it, vi } from 'vitest';
import { GroupsRepository } from '../repositories/groups.repository';

describe('GroupsRepository', () => {
  it('maps a member without uid to null', async () => {
    const get = vi.fn().mockResolvedValue({
      data: {
        cn: 'service',
        description: '',
        members: [
          { dn: 'uid=alice,ou=people,dc=example,dc=com', uid: 'alice' },
          { dn: 'ou=robots,dc=example,dc=com', uid: '' },
        ],
      },
    });
    const repository = new GroupsRepository({ get } as unknown as AxiosInstance);

    const group = await repository.get('service');

    expect(group.members).toEqual([
      { dn: 'uid=alice,ou=people,dc=example,dc=com', uid: 'alice' },
      { dn: 'ou=robots,dc=example,dc=com', uid: null },
    ]);
  });

  it('creates a group with member uids and encodes path parts', async () => {
    const post = vi.fn().mockResolvedValue({ data: { cn: 'team', description: 'Team', members: [] } });
    const del = vi.fn().mockResolvedValue({ data: null });
    const repository = new GroupsRepository({ post, delete: del } as unknown as AxiosInstance);

    await repository.create({ name: 'team', description: 'Team', memberUids: ['alice'] });
    await repository.removeMember('team', 'a.b');

    expect(post).toHaveBeenCalledWith('/groups', { cn: 'team', description: 'Team', members: ['alice'] });
    expect(del).toHaveBeenCalledWith('/groups/team/members/a.b');
  });
});
