import { AxiosError, AxiosHeaders } from 'axios';
import { describe, expect, it, vi } from 'vitest';
import { ApiError, errorMessage, toApiError } from '../api-error';
import { rejectWithApiError, setUnauthorizedHandler } from '../http-client';

function responseError(status: number, data: unknown): AxiosError {
  const config = { headers: new AxiosHeaders() };
  return new AxiosError('Request failed', 'ERR_BAD_REQUEST', config, null, {
    data, status, statusText: '', headers: {}, config,
  });
}

describe('toApiError', () => {
  it('reads code, message and groups from the backend body', () => {
    const error = toApiError(responseError(409, {
      error: 'sole_member', message: 'user is the only member of groups: solo', groups: ['solo'],
    }));

    expect(error).toBeInstanceOf(ApiError);
    expect(error.status).toBe(409);
    expect(error.code).toBe('sole_member');
    expect(error.groups).toEqual(['solo']);
    expect(errorMessage(error)).toBe('user is the only member of groups: solo');
  });

  it('marks a missing response as a network error', () => {
    const error = toApiError(new AxiosError('Network Error', 'ERR_NETWORK'));

    expect(error.code).toBe('network');
    expect(error.status).toBe(0);
  });

  it('survives a body that is not JSON', () => {
    const error = toApiError(responseError(502, '<html>Bad Gateway</html>'));

    expect(error.code).toBe('unknown');
    expect(error.status).toBe(502);
  });
});

describe('rejectWithApiError', () => {
  it('calls the unauthorized handler only for an expired session', async () => {
    const handler = vi.fn();
    setUnauthorizedHandler(handler);

    await expect(rejectWithApiError(responseError(401, { error: 'invalid_credentials', message: 'x' }))).rejects.toBeInstanceOf(ApiError);
    expect(handler).not.toHaveBeenCalled();

    await expect(rejectWithApiError(responseError(401, { error: 'unauthenticated', message: 'x' }))).rejects.toBeInstanceOf(ApiError);
    expect(handler).toHaveBeenCalledOnce();
  });
});
