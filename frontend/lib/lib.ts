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

export const StateColor = (state?: number | string): string => {
  const stateNum = typeof state === "string" ? parseInt(state) : state;
  
  switch (stateNum) {
    case 1:
      return "blue";
    case 101:
      return "orange";
    case 102:
      return "red";
    case 103:
      return "red";
    case 104:
      return "red";
    case 105:
      return "red";
    case 106:
      return "red";
    case 107:
      return "purple";
    case 200:
      return "green";
    case 300:
      return "red";
    default:
      return "gray";
  }
};

// Pretests mark verdicts that system testing will replace after the contest.
export const StateString = (state?: number | string, failedTest?: number | null, pretests?: boolean): string => {
  const stateNum = typeof state === "string" ? parseInt(state) : state;
  
  let baseString: string;
  switch (stateNum) {
    case 1:
      return "Тестируется";
    case 101:
      return "Ошибка компиляции";
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
      return pretests ? "Претесты пройдены" : "Принято";
    case 300:
      return "Дисквалифицировано";
    default:
      return "Неизвестно";
  }
  
  if (failedTest !== null && failedTest !== undefined && failedTest > 0) {
    return `${baseString} на ${pretests ? "претесте" : "тесте"} ${failedTest}`;
  }
  
  return baseString;
};

export const ShortVerdictString = (state?: number | string, failedTest?: number | null, pretests?: boolean): string => {
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
      return pretests ? "PP" : "AC";
    case 300:
      return "DQ";
    default:
      return "—";
  }
};

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

export const formatDate = (isoDate: string | undefined): string => {
  if (!isoDate) {
    return "—";
  }
  
  try {
    const date = new Date(isoDate);
    
    if (isNaN(date.getTime())) {
      return "—";
    }
    
    // Preserve the calendar date across client time zones.
    const day = String(date.getUTCDate()).padStart(2, '0');
    const month = String(date.getUTCMonth() + 1).padStart(2, '0');
    const year = date.getUTCFullYear();
    return `${day}.${month}.${year}`;
  } catch {
    return "—";
  }
};

export const ProblemTitle = (position?: number, title?: string): string => {
  const letter = numberToLetters(position);
  return title ? `${letter}. ${title}` : letter;
};

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
