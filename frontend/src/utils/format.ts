const OTP_PATTERN = /\b\d{4,8}\b/;

export function formatDateTime(value: string | Date | null | undefined): string {
  if (!value) return "-";
  const d = value instanceof Date ? value : new Date(value);
  if (isNaN(d.getTime())) return String(value);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getDate())}/${pad(d.getMonth() + 1)}/${d.getFullYear()}, ${pad(d.getHours())}.${pad(d.getMinutes())}.${pad(d.getSeconds())}`;
}

export function detectOtp(text: string | null | undefined): string | null {
  if (!text) return null;
  const match = text.match(OTP_PATTERN);
  return match ? match[0] : null;
}

