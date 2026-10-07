import { toApiError } from '@/common/services/api/api-error';

const LOGIN_MESSAGES: Record<string, string> = {
  invalid_credentials: 'Неверный uid или пароль',
  not_admin: 'Вход разрешён только участникам группы admins',
};

export function loginErrorMessage(error: unknown): string {
  const apiError = toApiError(error);
  return LOGIN_MESSAGES[apiError.code] ?? apiError.message;
}
