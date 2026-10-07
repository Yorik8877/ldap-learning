// Только пути внутри приложения: «//host» браузер понял бы как адрес другого сайта.
export function safeRedirectTarget(value: unknown): string {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//')) {
    return '/';
  }
  return value;
}
