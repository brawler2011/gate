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
}

export { parsePage };
