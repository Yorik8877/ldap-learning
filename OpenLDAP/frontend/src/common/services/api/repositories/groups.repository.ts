import type { AxiosInstance } from 'axios';
import type { GroupDto, GroupSummaryDto } from '../dtos';
import type { Group, GroupDraft, GroupSummary } from '../models';

export class GroupsRepository {
  private readonly http: AxiosInstance;

  constructor(http: AxiosInstance) {
    this.http = http;
  }

  async list(): Promise<GroupSummary[]> {
    const { data } = await this.http.get<GroupSummaryDto[]>('/groups');
    return data.map((dto) => ({ name: dto.cn, description: dto.description, memberCount: dto.memberCount }));
  }

  async get(name: string): Promise<Group> {
    const { data } = await this.http.get<GroupDto>(groupPath(name));
    return toGroup(data);
  }

  async create(draft: GroupDraft): Promise<Group> {
    const { data } = await this.http.post<GroupDto>('/groups', {
      cn: draft.name, description: draft.description, members: draft.memberUids,
    });
    return toGroup(data);
  }

  async remove(name: string): Promise<void> {
    await this.http.delete(groupPath(name));
  }

  async addMember(name: string, uid: string): Promise<void> {
    await this.http.post(`${groupPath(name)}/members`, { uid });
  }

  async removeMember(name: string, uid: string): Promise<void> {
    await this.http.delete(`${groupPath(name)}/members/${encodeURIComponent(uid)}`);
  }
}

function groupPath(name: string): string {
  return `/groups/${encodeURIComponent(name)}`;
}

function toGroup(dto: GroupDto): Group {
  return {
    name: dto.cn,
    description: dto.description,
    members: dto.members.map((member) => ({ dn: member.dn, uid: member.uid === '' ? null : member.uid })),
  };
}
