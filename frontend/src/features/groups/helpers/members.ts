import type { Group, User } from '@/common/services/api/models';

export function candidateMembers(users: User[], group: Group): User[] {
  const memberUids = new Set(group.members.map((member) => member.uid));
  return users.filter((user) => !memberUids.has(user.uid));
}
