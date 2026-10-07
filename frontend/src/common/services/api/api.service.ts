import { createHttpClient } from './http-client';
import { AuthRepository } from './repositories/auth.repository';
import { DirectoryRepository } from './repositories/directory.repository';
import { UsersRepository } from './repositories/users.repository';

const http = createHttpClient();

export const apiService = {
  auth: new AuthRepository(http),
  directory: new DirectoryRepository(http),
  users: new UsersRepository(http),
};
