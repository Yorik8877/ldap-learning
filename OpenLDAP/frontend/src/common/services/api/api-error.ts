import { isAxiosError } from 'axios';

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly groups: string[];

  constructor(status: number, code: string, message: string, groups: string[] = []) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.groups = groups;
  }
}

interface ErrorBody {
  error?: unknown;
  message?: unknown;
  groups?: unknown;
}

export function isApiError(value: unknown): value is ApiError {
  return value instanceof ApiError;
}

export function toApiError(error: unknown): ApiError {
  if (isApiError(error)) {
    return error;
  }
  if (!isAxiosError(error)) {
    return new ApiError(0, 'unknown', error instanceof Error ? error.message : String(error));
  }
  if (!error.response) {
    return new ApiError(0, 'network', 'Бэкенд недоступен');
  }
  const body: ErrorBody = isRecord(error.response.data) ? error.response.data : {};
  return new ApiError(
    error.response.status,
    typeof body.error === 'string' ? body.error : 'unknown',
    typeof body.message === 'string' ? body.message : error.message,
    Array.isArray(body.groups) ? body.groups.filter((name): name is string => typeof name === 'string') : [],
  );
}

export function errorMessage(error: unknown): string {
  return toApiError(error).message;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
