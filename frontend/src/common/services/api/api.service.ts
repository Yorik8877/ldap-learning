import { createHttpClient } from './http-client';
import { AuthRepository } from './repositories/auth.repository';
import { DirectoryRepository } from './repositories/directory.repository';

const http = createHttpClient();

export const apiService = {
  auth: new AuthRepository(http),
  directory: new DirectoryRepository(http),
};
