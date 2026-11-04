export function parseDecimalBigint(value: string): bigint | undefined {
  const trimmed = value.trim();
  if (trimmed === "") {
    return undefined;
  }
  if (!/^-?\d+$/.test(trimmed)) {
    return undefined;
  }
  return BigInt(trimmed);
}

export function formatDecimalBigint(value: bigint | undefined | null): string {
  if (value === undefined || value === null) {
    return "";
  }
  return value.toString();
}
