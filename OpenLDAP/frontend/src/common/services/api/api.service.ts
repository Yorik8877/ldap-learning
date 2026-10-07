import { createHttpClient } from './http-client';
import { AuthRepository } from './repositories/auth.repository';
import { DirectoryRepository } from './repositories/directory.repository';
import { GroupsRepository } from './repositories/groups.repository';
import { UsersRepository } from './repositories/users.repository';

const http = createHttpClient();

export const apiService = {
  auth: new AuthRepository(http),
  directory: new DirectoryRepository(http),
  groups: new GroupsRepository(http),
  users: new UsersRepository(http),
};
