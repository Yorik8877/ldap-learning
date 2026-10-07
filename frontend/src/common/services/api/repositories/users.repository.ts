import type { AxiosInstance } from 'axios';
import type { UserDetailsDto, UserDto } from '../dtos';
import type { User, UserChanges, UserDetails, UserDraft } from '../models';

export class UsersRepository {
  private readonly http: AxiosInstance;

  constructor(http: AxiosInstance) {
    this.http = http;
  }

  async list(): Promise<User[]> {
    const { data } = await this.http.get<UserDto[]>('/users');
    return data.map(toUser);
  }

  async get(uid: string): Promise<UserDetails> {
    const { data } = await this.http.get<UserDetailsDto>(userPath(uid));
    return { ...toUser(data), groups: data.groups };
  }

  async create(draft: UserDraft): Promise<User> {
    const { data } = await this.http.post<UserDto>('/users', {
      uid: draft.uid, ...toChangesDto(draft), password: draft.password,
    });
    return toUser(data);
  }

  async update(uid: string, changes: UserChanges): Promise<User> {
    const { data } = await this.http.put<UserDto>(userPath(uid), toChangesDto(changes));
    return toUser(data);
  }

  async setPassword(uid: string, password: string): Promise<void> {
    await this.http.put(`${userPath(uid)}/password`, { password });
  }

  async remove(uid: string): Promise<void> {
    await this.http.delete(userPath(uid));
  }
}

function userPath(uid: string): string {
  return `/users/${encodeURIComponent(uid)}`;
}

function toUser(dto: UserDto): User {
  return { uid: dto.uid, commonName: dto.cn, surname: dto.sn, emails: dto.mail };
}

function toChangesDto(changes: UserChanges): Omit<UserDto, 'uid'> {
  return { cn: changes.commonName, sn: changes.surname, mail: changes.emails };
}
