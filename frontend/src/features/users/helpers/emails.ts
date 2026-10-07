export function parseEmails(text: string): string[] {
  return text.split(/[\s,;]+/).filter((part) => part !== '');
}

export function formatEmails(emails: string[]): string {
  return emails.join(', ');
}
