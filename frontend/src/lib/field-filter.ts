// Field filters: space-separated `key=value` / `key!=value` terms on structured
// log lines, all of which must hold. A JSON object line is looked up by key or
// dot path (`http.status`); any other line is read as logfmt. This mirrors
// server/internal/models/fields.go, so live filtering in the browser and
// stored history filtered on the server agree.

const TERM_REGEX = /^([A-Za-z_@][A-Za-z0-9_.@-]*)(!?=)(.*)$/;
// The JSON number grammar, which parses the same way here and in Go.
const JSON_NUMBER_REGEX =
	/^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$/;
// A key a clicked value can be filtered by: one that round-trips as a term.
export const FIELD_KEY_REGEX = /^[A-Za-z_@][A-Za-z0-9_.@-]*$/;

export interface FieldTerm {
	// The term as typed, sent to the server as-is.
	raw: string;
	key: string;
	// Lowercased: values compare case-insensitively.
	value: string;
	// key!=value: the line lacks the field or holds another value.
	negate: boolean;
	// A numeric value equals a JSON number numerically, so 42 matches 42.0.
	number: number | null;
}

export interface SplitSearch {
	terms: FieldTerm[];
	// Everything that is not a term, searched as before.
	text: string;
}

export function splitFieldTerms(search: string): SplitSearch {
	const terms: FieldTerm[] = [];
	const rest: string[] = [];
	for (const token of splitTokens(search)) {
		const term = parseTerm(token);
		if (term) terms.push(term);
		else rest.push(token);
	}
	// Without terms the text is left exactly as typed.
	return { terms, text: terms.length === 0 ? search : rest.join(" ") };
}

function isSpace(c: string): boolean {
	return c === " " || c === "\t" || c === "\n" || c === "\r";
}

// Splits on whitespace outside double quotes.
function splitTokens(s: string): string[] {
	const tokens: string[] = [];
	let start = -1;
	let quoted = false;
	let escaped = false;
	for (let i = 0; i < s.length; i++) {
		const c = s[i];
		if (escaped) {
			escaped = false;
		} else if (quoted && c === "\\") {
			escaped = true;
		} else if (c === '"') {
			quoted = !quoted;
		} else if (!quoted && isSpace(c)) {
			if (start >= 0) {
				tokens.push(s.slice(start, i));
				start = -1;
			}
			continue;
		}
		if (start < 0) start = i;
	}
	if (start >= 0) tokens.push(s.slice(start));
	return tokens;
}

function parseTerm(token: string): FieldTerm | null {
	const match = TERM_REGEX.exec(token);
	if (!match) return null;
	let value = match[3];
	// An unclosed quote runs to the end of the term.
	if (value.startsWith('"')) value = readQuoted(value, 1)[0];
	return {
		raw: token,
		key: match[1],
		value: value.toLowerCase(),
		negate: match[2] === "!=",
		number: JSON_NUMBER_REGEX.test(value) ? Number(value) : null,
	};
}

// Reads a double-quoted value whose opening quote is just before s[start]. A
// backslash takes the next character literally. Returns the value and the
// index past the closing quote.
function readQuoted(s: string, start: number): [string, number] {
	let value = "";
	let i = start;
	for (; i < s.length; i++) {
		const c = s[i];
		if (c === "\\" && i + 1 < s.length) {
			value += s[++i];
		} else if (c === '"') {
			return [value, i + 1];
		} else {
			value += c;
		}
	}
	return [value, i];
}

// Formats a clicked value as a term value, quoting it when it would not
// survive as a bare token.
export function formatFieldTerm(key: string, value: string): string {
	if (value === "" || /[\s"\\]/.test(value)) {
		return `${key}="${value.replace(/["\\]/g, "\\$&")}"`;
	}
	return `${key}=${value}`;
}

// A field value as filters compare it. Booleans and null compare as their
// JSON text, as they do in Go.
type FieldValue =
	| { kind: "text"; lower: string }
	| { kind: "number"; number: number }
	| { kind: "object"; fields: Map<string, FieldValue> }
	| { kind: "array" };

// The fields of one line: a JSON object's, which nest, or logfmt pairs.
export interface LineFields {
	fields: Map<string, FieldValue>;
	nested: boolean;
}

// JSON.parse revives values bottom-up, so an object's properties are already
// FieldValues when the object itself arrives.
function reviveFieldValue(
	_key: string,
	value:
		| Record<string, FieldValue>
		| FieldValue[]
		| string
		| number
		| boolean
		| null,
): FieldValue {
	if (value === null || value === true || value === false) {
		return { kind: "text", lower: String(value) };
	}
	if (Array.isArray(value)) return { kind: "array" };
	if (Number.isFinite(value)) return { kind: "number", number: Number(value) };
	if (value instanceof Object) {
		return { kind: "object", fields: new Map(Object.entries(value)) };
	}
	return { kind: "text", lower: String(value).toLowerCase() };
}

export function parseLineFields(message: string): LineFields {
	const trimmed = message.trim();
	if (trimmed.startsWith("{")) {
		try {
			// SAFETY: reviveFieldValue turns every parsed value into a FieldValue.
			const parsed = JSON.parse(trimmed, reviveFieldValue) as FieldValue;
			if (parsed.kind === "object") {
				return { fields: parsed.fields, nested: true };
			}
		} catch {}
	}
	return { fields: parseLogfmt(trimmed), nested: false };
}

// Reads the key=value and key="quoted value" pairs of a line and skips
// everything else. A repeated key keeps its last value.
function parseLogfmt(s: string): Map<string, FieldValue> {
	const pairs = new Map<string, FieldValue>();
	let i = 0;
	while (i < s.length) {
		if (isSpace(s[i])) {
			i++;
			continue;
		}
		const start = i;
		while (i < s.length && !isSpace(s[i]) && s[i] !== "=" && s[i] !== '"') {
			i++;
		}
		if (i === start || i === s.length || s[i] !== "=") {
			while (i < s.length && !isSpace(s[i])) i++;
			continue;
		}
		const key = s.slice(start, i);
		i++;
		let value: string;
		if (i < s.length && s[i] === '"') {
			[value, i] = readQuoted(s, i + 1);
		} else {
			const valueStart = i;
			while (i < s.length && !isSpace(s[i])) i++;
			value = s.slice(valueStart, i);
		}
		pairs.set(key, { kind: "text", lower: value.toLowerCase() });
	}
	return pairs;
}

// Finds key as a top-level JSON key first, so "log.level" names a dotted key
// when the line has one, then as a dot path.
function lookup(line: LineFields, key: string): FieldValue | undefined {
	const literal = line.fields.get(key);
	if (literal || !line.nested) return literal;
	let current: FieldValue | undefined = {
		kind: "object",
		fields: line.fields,
	};
	for (const segment of key.split(".")) {
		if (current?.kind !== "object") return undefined;
		current = current.fields.get(segment);
	}
	return current;
}

function equals(term: FieldTerm, value: FieldValue | undefined): boolean {
	switch (value?.kind) {
		case "text":
			return value.lower === term.value;
		case "number":
			return value.number === term.number;
		default:
			return false; // absent, an object or an array
	}
}

// Rules a line out before it is parsed. Without backslash escapes a line
// holds its keys and values verbatim, so one missing the last segment of a
// key, or a non-numeric value, cannot match that term. Numbers are skipped:
// 42 can be written 42.0.
function mayMatch(terms: FieldTerm[], message: string): boolean {
	if (message.includes("\\")) return true;
	for (const term of terms) {
		const key = term.key.slice(term.key.lastIndexOf(".") + 1);
		if (!term.negate && !message.includes(key)) return false;
	}
	let lower: string | undefined;
	for (const term of terms) {
		if (term.negate || term.number !== null) continue;
		lower ??= message.toLowerCase();
		if (!lower.includes(term.value)) return false;
	}
	return true;
}

// Reports whether message satisfies every term. readFields is only called
// for lines the cheap check cannot rule out, so callers can cache the parse.
export function matchFieldTerms(
	terms: FieldTerm[],
	message: string,
	readFields: () => LineFields,
): boolean {
	if (terms.length === 0) return true;
	if (!mayMatch(terms, message)) return false;
	const fields = readFields();
	return terms.every(
		(term) => equals(term, lookup(fields, term.key)) !== term.negate,
	);
}
