import { createHttpClient } from './http-client';
import { AuthRepository } from './repositories/auth.repository';

const http = createHttpClient();

export const apiService = {
  auth: new AuthRepository(http),
};
