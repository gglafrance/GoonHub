// Date display following the user's date format setting.
// "auto" uses the browser locale; the other formats are fixed patterns with English month names.

export const DATE_FORMATS = [
    'auto',
    'DD/MM/YYYY',
    'MM/DD/YYYY',
    'YYYY-MM-DD',
    'DD.MM.YYYY',
    'D MMM YYYY',
    'MMM D, YYYY',
] as const;

export type DateFormat = (typeof DATE_FORMATS)[number];

export const DEFAULT_DATE_FORMAT: DateFormat = 'auto';

export interface FormatDateOptions {
    /** Append the time (hours and minutes, browser locale) */
    time?: boolean;
    /** Include seconds in the time */
    seconds?: boolean;
    /** Calendar dates (release dates, YYYY-MM-DD keys): read and shown in UTC so the day never shifts */
    utc?: boolean;
}

export type DateInput = string | number | Date | null | undefined;

const formatterCache = new Map<string, Intl.DateTimeFormat>();

function formatter(locale: string | undefined, options: Intl.DateTimeFormatOptions) {
    const key = `${locale ?? ''}|${JSON.stringify(options)}`;
    let f = formatterCache.get(key);
    if (!f) {
        f = new Intl.DateTimeFormat(locale, options);
        formatterCache.set(key, f);
    }
    return f;
}

export function isDateFormat(value: unknown): value is DateFormat {
    return typeof value === 'string' && (DATE_FORMATS as readonly string[]).includes(value);
}

/** Parses a date value; plain YYYY-MM-DD strings are read as UTC calendar days. */
export function toDate(value: DateInput): Date | null {
    if (value === null || value === undefined || value === '') return null;
    let d: Date;
    if (value instanceof Date) d = value;
    else if (typeof value === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(value))
        d = new Date(`${value}T00:00:00Z`);
    else d = new Date(value);
    return Number.isNaN(d.getTime()) ? null : d;
}

function formatDatePart(d: Date, format: DateFormat, timeZone: string | undefined): string {
    if (format === 'auto') {
        return formatter(undefined, {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            timeZone,
        }).format(d);
    }
    const parts: Record<string, string> = {};
    for (const p of formatter('en-US', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        timeZone,
    }).formatToParts(d)) {
        parts[p.type] = p.value;
    }
    const monthName = formatter('en-US', { month: 'short', timeZone }).format(d);
    const tokens: Record<string, string> = {
        YYYY: parts.year ?? '',
        MMM: monthName,
        MM: parts.month ?? '',
        DD: parts.day ?? '',
        D: String(Number(parts.day ?? '')),
    };
    return format.replace(/YYYY|MMM|MM|DD|D/g, (t) => tokens[t] ?? t);
}

/** Formats a date (and optionally its time) with the given date format. Returns '' for empty input. */
export function formatDateWith(
    value: DateInput,
    format: DateFormat,
    options: FormatDateOptions = {},
): string {
    const d = toDate(value);
    if (!d) return '';
    const timeZone = options.utc ? 'UTC' : undefined;
    const date = formatDatePart(d, isDateFormat(format) ? format : DEFAULT_DATE_FORMAT, timeZone);
    if (!options.time) return date;
    const time = formatter(undefined, {
        hour: '2-digit',
        minute: '2-digit',
        second: options.seconds ? '2-digit' : undefined,
        timeZone,
    }).format(d);
    return `${date} ${time}`;
}
