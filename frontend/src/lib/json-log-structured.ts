export type JsonFieldOperator =
  | "eq"
  | "neq"
  | "contains"
  | "gt"
  | "gte"
  | "lt"
  | "lte";

export type JsonSortDirection = "asc" | "desc";

export function extractTopLevelJsonObject(
  text: string
): Record<string, unknown> | null {
  const trimmed = text.trim();
  if (!trimmed.startsWith("{")) return null;

  try {
    const parsed = JSON.parse(trimmed);
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>;
    }
    return null;
  } catch {
    return null;
  }
}

function coerceToComparable(value: unknown): string | number | boolean | null {
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
    return value;
  }
  if (value === null || value === undefined) return null;
  return JSON.stringify(value);
}

function coerceFilterValue(raw: string): string | number | boolean {
  const trimmed = raw.trim();
  if (trimmed.toLowerCase() === "true") return true;
  if (trimmed.toLowerCase() === "false") return false;
  const asNumber = Number(trimmed);
  if (!Number.isNaN(asNumber)) return asNumber;
  return trimmed;
}

export function matchesJsonFieldFilter(
  fieldValue: unknown,
  operator: JsonFieldOperator,
  rawFilterValue: string
): boolean {
  const comparable = coerceToComparable(fieldValue);
  if (comparable === null) return false;

  const filterValue = coerceFilterValue(rawFilterValue);

  if (operator === "contains") {
    return String(comparable).toLowerCase().includes(rawFilterValue.toLowerCase());
  }

  if (operator === "eq" || operator === "neq") {
    let equal = false;
    if (typeof comparable === "number" && typeof filterValue === "number") {
      equal = comparable === filterValue;
    } else if (typeof comparable === "boolean" && typeof filterValue === "boolean") {
      equal = comparable === filterValue;
    } else {
      equal = String(comparable).toLowerCase() === String(filterValue).toLowerCase();
    }
    return operator === "eq" ? equal : !equal;
  }

  const left = Number(comparable);
  const right = Number(filterValue);
  if (Number.isNaN(left) || Number.isNaN(right)) return false;

  if (operator === "gt") return left > right;
  if (operator === "gte") return left >= right;
  if (operator === "lt") return left < right;
  return left <= right;
}

export function compareJsonFieldValues(
  aValue: unknown,
  bValue: unknown,
  direction: JsonSortDirection
): number {
  const aComparable = coerceToComparable(aValue);
  const bComparable = coerceToComparable(bValue);

  if (aComparable === null && bComparable === null) return 0;
  if (aComparable === null) return 1;
  if (bComparable === null) return -1;

  let base = 0;
  if (typeof aComparable === "number" && typeof bComparable === "number") {
    base = aComparable - bComparable;
  } else if (typeof aComparable === "boolean" && typeof bComparable === "boolean") {
    base = Number(aComparable) - Number(bComparable);
  } else {
    base = String(aComparable).localeCompare(String(bComparable));
  }

  return direction === "asc" ? base : -base;
}
