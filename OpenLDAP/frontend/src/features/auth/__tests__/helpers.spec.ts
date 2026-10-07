import { describe, expect, it } from 'vitest';
import { ApiError } from '@/common/services/api/api-error';
import { loginErrorMessage } from '../helpers/login-error';
import { safeRedirectTarget } from '../helpers/redirect-target';

describe('loginErrorMessage', () => {
  it('distinguishes a wrong password from a missing admin membership', () => {
    expect(loginErrorMessage(new ApiError(401, 'invalid_credentials', 'x'))).toBe('Неверный uid или пароль');
    expect(loginErrorMessage(new ApiError(403, 'not_admin', 'x'))).toBe('Вход разрешён только участникам группы admins');
  });

  it('falls back to the backend message', () => {
    expect(loginErrorMessage(new ApiError(503, 'directory_unavailable', 'directory unavailable'))).toBe('directory unavailable');
  });
});

describe('safeRedirectTarget', () => {
  it('keeps internal paths only', () => {
    expect(safeRedirectTarget('/users/alice')).toBe('/users/alice');
    expect(safeRedirectTarget('//evil.example')).toBe('/');
    expect(safeRedirectTarget('https://evil.example')).toBe('/');
    expect(safeRedirectTarget(undefined)).toBe('/');
    expect(safeRedirectTarget(['/users'])).toBe('/');
  });
});
