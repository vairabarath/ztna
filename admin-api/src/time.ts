export function toProtoTimestamp(input: Date): { seconds: number; nanos: number } {
  const millis = input.getTime();
  const seconds = Math.floor(millis / 1000);
  const nanos = (millis % 1000) * 1_000_000;
  return { seconds, nanos };
}

export function protoTimestampToISO(input: unknown): string {
  if (!input || typeof input !== "object") {
    return new Date(0).toISOString();
  }
  const src = input as { seconds?: number | string; nanos?: number | string };
  const secondsRaw = src.seconds ?? 0;
  const nanosRaw = src.nanos ?? 0;
  const seconds = typeof secondsRaw === "string" ? Number(secondsRaw) : secondsRaw;
  const nanos = typeof nanosRaw === "string" ? Number(nanosRaw) : nanosRaw;
  const millis = seconds * 1000 + Math.floor(nanos / 1_000_000);
  return new Date(millis).toISOString();
}
