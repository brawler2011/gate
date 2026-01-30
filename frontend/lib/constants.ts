// Programming languages configuration
export const LANGUAGES = {
  golang: { id: 10, name: 'Go', extension: '.go' },
  cpp: { id: 20, name: 'C++', extension: '.cpp' },
  python: { id: 30, name: 'Python', extension: '.py' },
} as const;

// Language name to ID mapping for API calls
export const LANGUAGE_MAP: Record<string, number> = {
  golang: LANGUAGES.golang.id,
  cpp: LANGUAGES.cpp.id,
  python: LANGUAGES.python.id,
};

// Contest layout dimensions
export const CONTEST_CONTENT_MAX_WIDTH = '1080px';    // Контент (таблицы, формы)
export const CONTEST_PROBLEM_MAX_WIDTH = '800px';
export const CONTEST_SIDEBAR_LEFT_WIDTH = '250px';
export const CONTEST_SIDEBAR_RIGHT_WIDTH = '520px';
export const CONTEST_INFO_PANEL_WIDTH = '370px';     // Панель информации о контесте (половина от hotbar)

