import { describe, expect, it } from 'vitest';
import type { Group, User } from '@/common/services/api/models';
import { candidateMembers } from '../helpers/members';

function person(uid: string): User {
  return { uid, commonName: uid, surname: uid, emails: [] };
}

describe('candidateMembers', () => {
  it('offers only users who are not in the group yet', () => {
    const group: Group = {
      name: 'team',
      description: '',
      members: [
        { dn: 'uid=alice,ou=people,dc=example,dc=com', uid: 'alice' },
        { dn: 'ou=robots,dc=example,dc=com', uid: null },
      ],
    };

    expect(candidateMembers([person('alice'), person('bob')], group).map((user) => user.uid)).toEqual(['bob']);
  });
});
