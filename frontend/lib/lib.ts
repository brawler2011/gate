/**
 * Converts a number to alphabetic letters (1 -> A, 2 -> B, etc.)
 */
export function numberToLetters(num?: number | null): string {
  if (!num || num <= 0) return "?";
  
  let result = "";
  let n = num;
  
  while (n > 0) {
    n--;
    result = String.fromCharCode(65 + (n % 26)) + result;
    n = Math.floor(n / 26);
  }
  
  return result;
}

/**
 * Get color for submission state
 */
export function StateColor(state?: number | string): string {
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
    case 200:
      return "green"; // Accepted
    default:
      return "gray";
  }
}

/**
 * Get string representation of submission state
 * @param state - submission state code
 * @param failedTest - optional test number where submission failed (1-indexed)
 */
export function StateString(state?: number | string, failedTest?: number | null): string {
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
    case 200:
      return "Принято"; // Accepted - no failed test
    default:
      return "Неизвестно";
  }
  
  // Add test number if available (for error states that happen on specific tests)
  if (failedTest != null && failedTest > 0) {
    return `${baseString} на тесте ${failedTest}`;
  }
  
  return baseString;
}

export const isValidUUIDV4 = (str: string): boolean => {
    const uuidV4Regex = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
    return uuidV4Regex.test(str);
}

/**
 * Format ISO timestamp to readable format
 */
export function TimeBeautify(timestamp?: string): string {
  if (!timestamp) return "—";
  
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
}

/**
 * Convert language code to display string
 * Language mapping: golang = 10, cpp = 20, python = 30
 */
export function LangString(language?: number): string {
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
}

/**
 * Convert language code to syntax highlighter language name
 * Language mapping: golang = 10, cpp = 20, python = 30
 */
export function LangNameToString(language?: number): string {
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
}

/**
 * Format problem title with position letter
 */
export function ProblemTitle(position?: number, title?: string): string {
  const letter = numberToLetters(position);
  return title ? `${letter}. ${title}` : letter;
}

/**
 * Get color for user role badge
 */
export function getRoleColor(role: string): string {
  switch (role?.toLowerCase()) {
    case "admin":
      return "red";
    case "moderator":
      return "blue";
    case "user":
      return "gray";
    default:
      return "gray";
  }
}
