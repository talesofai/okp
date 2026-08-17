const dateTimeFormatter = new Intl.DateTimeFormat("zh-CN", {
  year: "numeric",
  month: "short",
  day: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

const dateFormatter = new Intl.DateTimeFormat("zh-CN", {
  month: "short",
  day: "numeric",
});

const yearDateFormatter = new Intl.DateTimeFormat("zh-CN", {
  year: "numeric",
  month: "short",
  day: "numeric",
});

const relativeTimeFormatter = new Intl.RelativeTimeFormat("zh-CN", {
  numeric: "auto",
});

function parseDate(value?: string): Date | null {
  if (!value) return null;
  const dateOnly = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  const date = dateOnly
    ? new Date(Number(dateOnly[1]), Number(dateOnly[2]) - 1, Number(dateOnly[3]))
    : new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

export function formatDateTime(value?: string): string {
  const date = parseDate(value);
  return date ? dateTimeFormatter.format(date) : "--";
}

export function formatShortDate(value?: string): string {
  const date = parseDate(value);
  return date ? dateFormatter.format(date) : "--";
}

export function formatCompactDate(value?: string, now = new Date()): string {
  const date = parseDate(value);
  if (!date) return "--";
  return date.getFullYear() === now.getFullYear()
    ? dateFormatter.format(date)
    : yearDateFormatter.format(date);
}

export function formatRelativeTime(value?: string, now = new Date()): string {
  const date = parseDate(value);
  if (!date) return "暂无记录";

  const differenceMs = date.getTime() - now.getTime();
  const absoluteMs = Math.abs(differenceMs);
  const minute = 60_000;
  const hour = 60 * minute;
  const day = 24 * hour;

  if (absoluteMs < minute) return "刚刚";
  if (absoluteMs < hour) {
    return relativeTimeFormatter.format(Math.round(differenceMs / minute), "minute");
  }
  if (absoluteMs < day) {
    return relativeTimeFormatter.format(Math.round(differenceMs / hour), "hour");
  }
  if (absoluteMs < 7 * day) {
    return relativeTimeFormatter.format(Math.round(differenceMs / day), "day");
  }
  if (absoluteMs < 30 * day) {
    return relativeTimeFormatter.format(Math.round(differenceMs / (7 * day)), "week");
  }
  if (absoluteMs < 365 * day) {
    return relativeTimeFormatter.format(Math.round(differenceMs / (30 * day)), "month");
  }
  return relativeTimeFormatter.format(Math.round(differenceMs / (365 * day)), "year");
}
