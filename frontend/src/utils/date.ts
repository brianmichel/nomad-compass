const RELATIVE_THRESHOLDS = [
  { limit: 365 * 24 * 60 * 60 * 1000, unit: 'year' as const },
  { limit: 30 * 24 * 60 * 60 * 1000, unit: 'month' as const },
  { limit: 7 * 24 * 60 * 60 * 1000, unit: 'week' as const },
  { limit: 24 * 60 * 60 * 1000, unit: 'day' as const },
  { limit: 60 * 60 * 1000, unit: 'hour' as const },
  { limit: 60 * 1000, unit: 'minute' as const },
];

export function formatTimestamp(value?: string | null) {
  if (!value) {
    return 'Awaiting first poll';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toLocaleString();
}

export function formatRelativeTime(value?: string | null, now: Date = new Date()) {
  if (!value) {
    return 'Awaiting first poll';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  const diff = date.getTime() - now.getTime();
  const absDiff = Math.abs(diff);

  const formatter = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });

  for (const threshold of RELATIVE_THRESHOLDS) {
    if (absDiff >= threshold.limit) {
      const amount = Math.round(diff / threshold.limit);
      return formatter.format(amount, threshold.unit);
    }
  }

  const seconds = Math.round(diff / 1000);
  if (Math.abs(seconds) < 1) {
    return 'just now';
  }
  return formatter.format(seconds, 'second');
}
