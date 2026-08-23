/**
 * Converts a number to alphabetic letters (1 -> A, 2 -> B, etc.)
 */
export const numberToLetters = (num?: number | null): string => {
  if (!num || num <= 0) {
    return "?";
  }
  
  let result = "";
  let n = num;
  
  while (n > 0) {
    n--;
    result = String.fromCharCode(65 + (n % 26)) + result;
    n = Math.floor(n / 26);
  }
  
  return result;
};

/**
 * Converts alphabetic letters to a 1-based number (A -> 1, B -> 2, ..., Z -> 26, AA -> 27, etc.)
 * Strictly validates that input contains only uppercase English letters.
 */
export const lettersToNumber = (str?: string | null): number => {
  if (!str) {
    return 0;
  }
  
  const trimmed = str.trim();
  if (!/^[A-Z]+$/.test(trimmed)) {
    return 0;
  }
  
  let result = 0;
  for (let i = 0; i < trimmed.length; i++) {
    result = result * 26 + (trimmed.charCodeAt(i) - 64);
  }
  
  return result;
};

/**
 * Get color for submission state
 */
export const StateColor = (state?: number | string): string => {
  const stateNum = typeof state === "string" ? parseInt(state) : state;
  
  switch (stateNum) {
    case 1:
      return "blue"; // Saved to DB
    case 101:
      return "orange"; // Compilation Error
    case 102:
      return "red"; // Time Limit Exceeded
    case 103:
      return "red"; // Memory Limit Exceeded
    case 104:
      return "red"; // Runtime Error
    case 105:
      return "red"; // Presentation Error
    case 106:
      return "red"; // Wrong Answer
    case 107:
      return "purple"; // Internal Error
    case 200:
      return "green"; // Accepted
    case 300:
      return "red"; // Disqualified
    default:
      return "gray";
  }
};

/**
 * Get string representation of submission state
 * @param state - submission state code
 * @param failedTest - optional test number where submission failed (1-indexed)
 */
export const StateString = (state?: number | string, failedTest?: number | null): string => {
  const stateNum = typeof state === "string" ? parseInt(state) : state;
  
  let baseString: string;
  switch (stateNum) {
    case 1:
      return "Тестируется"; // No test number for "testing" state
    case 101:
      return "Ошибка компиляции"; // Compilation error - no specific test
    case 102:
      baseString = "Превышено время исполнения";
      break;
    case 103:
      baseString = "Превышено ограничение памяти";
      break;
    case 104:
      baseString = "Ошибка исполнения";
      break;
    case 105:
      baseString = "Ошибка форматирования";
      break;
    case 106:
      baseString = "Неправильный ответ";
      break;
    case 107:
      baseString = "Ошибка тестирующей системы";
      break;
    case 200:
      return "Принято"; // Accepted - no failed test
    case 300:
      return "Дисквалифицировано";
    default:
      return "Неизвестно";
  }
  
  // Add test number if available (for error states that happen on specific tests)
  if (failedTest !== null && failedTest !== undefined && failedTest > 0) {
    return `${baseString} на тесте ${failedTest}`;
  }
  
  return baseString;
};

/**
 * Short Codeforces-style verdict representation (e.g., WA5, TLE50, AC, CE, RE, DQ)
 */
export const ShortVerdictString = (state?: number | string, failedTest?: number | null): string => {
  const stateNum = typeof state === "string" ? parseInt(state) : state;
  const testSuffix = failedTest !== null && failedTest !== undefined && failedTest > 0 ? `${failedTest}` : "";

  switch (stateNum) {
    case 1:
      return "Тестируется";
    case 101:
      return "CE";
    case 102:
      return `TLE${testSuffix}`;
    case 103:
      return `MLE${testSuffix}`;
    case 104:
      return `RE${testSuffix}`;
    case 105:
      return `PE${testSuffix}`;
    case 106:
      return `WA${testSuffix}`;
    case 107:
      return "IE";
    case 200:
      return "AC";
    case 300:
      return "DQ";
    default:
      return "—";
  }
};

/**
 * Format ISO timestamp to readable format
 */
export const TimeBeautify = (timestamp?: string): string => {
  if (!timestamp) {
    return "—";
  }
  
  try {
    const date = new Date(timestamp);
    return date.toLocaleString("ru-RU", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return timestamp;
  }
};

/**
 * Formats an ISO date string to "dd.MM.yyyy" format
 * @param isoDate - ISO date string (e.g., "2025-01-18")
 * @returns Formatted date string (e.g., "18.01.2025")
 */
export const formatDate = (isoDate: string | undefined): string => {
  if (!isoDate) {
    return "—";
  }
  
  try {
    const date = new Date(isoDate);
    
    // Check for Invalid Date
    if (isNaN(date.getTime())) {
      return "—";
    }
    
    // Use UTC methods to avoid timezone issues
    const day = String(date.getUTCDate()).padStart(2, '0');
    const month = String(date.getUTCMonth() + 1).padStart(2, '0');
    const year = date.getUTCFullYear();
    return `${day}.${month}.${year}`;
  } catch {
    return "—";
  }
};

/**
 * Convert language code to display string
 * Language mapping: golang = 10, cpp = 20, python = 30
 */
export const LangString = (language?: number): string => {
  switch (language) {
    case 10:
      return "Go";
    case 20:
      return "C++";
    case 30:
      return "Python";
    default:
      return "Unknown";
  }
};

/**
 * Convert language code to syntax highlighter language name
 * Language mapping: golang = 10, cpp = 20, python = 30
 */
export const LangNameToString = (language?: number): string => {
  switch (language) {
    case 10:
      return "go";
    case 20:
      return "cpp";
    case 30:
      return "python";
    default:
      return "text";
  }
};

/**
 * Format problem title with position letter
 */
export const ProblemTitle = (position?: number, title?: string): string => {
  const letter = numberToLetters(position);
  return title ? `${letter}. ${title}` : letter;
};

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
export const parsePage = (value: unknown): number | null => {
  if (value === undefined) {
    return 1;
  }

  const num = Number(value);
  if (Number.isInteger(num) && num > 0) {
    return num;
  }

  return null;
};
