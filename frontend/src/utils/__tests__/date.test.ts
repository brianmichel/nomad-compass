import { describe, expect, it } from 'vitest';
import { formatRelativeTime, formatTimestamp } from '@/utils/date';

describe('formatRelativeTime', () => {
  const now = new Date('2024-01-01T00:00:00Z');

  it('returns human readable values for past timestamps', () => {
    expect(formatRelativeTime('2023-12-31T00:00:00Z', now)).toBe('1 day ago');
  });

  it('returns human readable values for future timestamps', () => {
    expect(formatRelativeTime('2024-01-01T01:00:00Z', now)).toBe('in 1 hour');
  });

  it('falls back to awaiting copy when value is missing', () => {
    expect(formatRelativeTime(undefined, now)).toBe('Awaiting first poll');
  });

  it('returns original value when timestamp is invalid', () => {
    expect(formatRelativeTime('not-a-date', now)).toBe('not-a-date');
  });
});

describe('formatTimestamp', () => {
  it('returns awaiting copy when value is missing', () => {
    expect(formatTimestamp(null)).toBe('Awaiting first poll');
  });

  it('returns the original value when the timestamp cannot be parsed', () => {
    expect(formatTimestamp('not-a-date')).toBe('not-a-date');
  });

  it('formats valid timestamps using locale aware formatting', () => {
    const result = formatTimestamp('2024-01-01T00:00:00Z');
    expect(result).toContain('2024');
  });
});
