import axios, { type AxiosInstance } from 'axios';
import { toApiError } from './api-error';

let unauthorizedHandler: () => void = () => {};

export function setUnauthorizedHandler(handler: () => void): void {
  unauthorizedHandler = handler;
}

// invalid_credentials — ответ на неверный пароль при входе, а не истёкшая сессия,
// поэтому на страницу входа отправляет только unauthenticated.
export function rejectWithApiError(error: unknown): Promise<never> {
  const apiError = toApiError(error);
  if (apiError.code === 'unauthenticated') {
    unauthorizedHandler();
  }
  return Promise.reject(apiError);
}

export function createHttpClient(): AxiosInstance {
  const client = axios.create({ baseURL: '/api', withCredentials: true });
  client.interceptors.response.use((response) => response, rejectWithApiError);
  return client;
}
