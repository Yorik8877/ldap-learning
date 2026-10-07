import type { AxiosInstance } from 'axios';
import type { CurrentUserDto } from '../dtos';
import type { CurrentUser } from '../models';

export class AuthRepository {
  private readonly http: AxiosInstance;

  constructor(http: AxiosInstance) {
    this.http = http;
  }

  async login(uid: string, password: string): Promise<CurrentUser> {
    const { data } = await this.http.post<CurrentUserDto>('/auth/login', { uid, password });
    return toCurrentUser(data);
  }

  async logout(): Promise<void> {
    await this.http.post('/auth/logout');
  }

  async me(): Promise<CurrentUser> {
    const { data } = await this.http.get<CurrentUserDto>('/auth/me');
    return toCurrentUser(data);
  }
}

function toCurrentUser(dto: CurrentUserDto): CurrentUser {
  return { uid: dto.uid, commonName: dto.cn };
}
