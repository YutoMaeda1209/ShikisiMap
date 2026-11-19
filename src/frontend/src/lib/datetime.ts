// Conversions between ISO 8601 and the value of <input type="datetime-local">.

function pad(n: number): string {
  return String(n).padStart(2, "0");
}

/** The local date and time as "YYYY-MM-DDTHH:mm". */
export function toLocalInputValue(date: Date): string {
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` +
    `T${pad(date.getHours())}:${pad(date.getMinutes())}`
  );
}

/** The ISO 8601 form of a datetime-local value, or null when it is not a date. */
export function fromLocalInputValue(value: string): string | null {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date.toISOString();
}

const formatter = new Intl.DateTimeFormat("ja-JP", { dateStyle: "medium", timeStyle: "short" });

export function formatDateTime(iso: string): string {
  return formatter.format(new Date(iso));
}
