const UNIT_MULTIPLIERS: Record<string, number> = {
  b: 1,
  k: 1024,
  m: 1024 ** 2,
  g: 1024 ** 3,
};

// Parses a human memory value like "512m", "1.5g", "1073741824" into bytes.
// Empty input means unlimited (0). Returns null when the input is invalid.
export function parseMemoryInput(input: string): number | null {
  const trimmed = input.trim().toLowerCase();
  if (trimmed === "") {
    return 0;
  }

  const match = /^(\d+(?:\.\d+)?)\s*(b|kb?|mb?|gb?)?$/.exec(trimmed);
  if (!match) {
    return null;
  }

  const value = Number.parseFloat(match[1]);
  const unit = match[2]?.[0] ?? "b";
  return Math.round(value * UNIT_MULTIPLIERS[unit]);
}

// Formats bytes back into the largest exact unit, e.g. 536870912 -> "512m".
// Returns "" for 0 or negative values (unlimited).
export function formatMemoryBytes(bytes: number): string {
  if (bytes <= 0) {
    return "";
  }

  const units: Array<[string, number]> = [
    ["g", 1024 ** 3],
    ["m", 1024 ** 2],
    ["k", 1024],
  ];
  for (const [suffix, size] of units) {
    if (bytes % size === 0) {
      return `${bytes / size}${suffix}`;
    }
  }
  return `${bytes}`;
}
