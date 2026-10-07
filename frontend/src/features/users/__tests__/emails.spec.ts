import { describe, expect, it } from 'vitest';
import { formatEmails, parseEmails } from '../helpers/emails';

describe('emails', () => {
  it('splits by commas, semicolons and spaces and drops empties', () => {
    expect(parseEmails(' a@example.com, b@example.com;c@example.com  ')).toEqual(['a@example.com', 'b@example.com', 'c@example.com']);
    expect(parseEmails('   ')).toEqual([]);
  });

  it('formats as a comma-separated list', () => {
    expect(formatEmails(['a@example.com', 'b@example.com'])).toBe('a@example.com, b@example.com');
  });
});
