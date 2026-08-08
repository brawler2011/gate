/**
  * Parses page number.
  *
  * @param value Raw page number
  * @returns Valid page number (`>=1`) or `null`
  *
  * @example 
  * parsePage(undefined) // 1 (default)
  * parsePage("3") // 3
  * parsePage("abcd") // null
  * parsePage("-5") // null
  */
const parsePage = (value: unknown): number | null => {
  if (value === undefined) {
    return 1;
  }

  const num = Number(value);
  if (Number.isInteger(num) && num > 0) {
    return num;
  }

  return null;
};

const UUID_V4_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

/**
 * Parses and validates UUID v4 string.
 *
 * @param value Raw ID value
 * @returns Valid UUID v4 string or `null`
 *
 * @example
 * parseId("f47ac10b-58cc-4372-a567-0e02b2c3d479") // "f47ac10b-58cc-4372-a567-0e02b2c3d479"
 * parseId("invalid-id") // null
 * parseId(undefined) // null
 */
const parseId = (value: unknown): string | null => {
  if (typeof value !== "string") {
    return null;
  }

  if (UUID_V4_REGEX.test(value)) {
    return value;
  }

  return null;
};

export { parsePage, parseId };

