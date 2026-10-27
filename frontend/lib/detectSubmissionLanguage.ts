export type SubmissionLanguage = "python" | "cpp" | "golang";

// Bound work for large pasted files. Strong syntax signals are required to avoid
// guessing from generic expressions, comments, or a file extension.
export const detectSubmissionLanguage = (source: string): SubmissionLanguage | null => {
  const sample = source.slice(0, 32768).replace(
    /"""[\s\S]*?"""|'''[\s\S]*?'''|"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|`[^`]*`|\/\*[\s\S]*?\*\/|\/\/[^\n]*|#[^\n]*/g,
    (token) => /^#\s*include\b/.test(token) ? token : "\"\"",
  );

  const signals: Record<SubmissionLanguage, RegExp[]> = {
    python: [
      /^\s*(?:async\s+)?def\s+\w+\s*\([^\n]*\)\s*(?:->[^\n]+)?\s*:/m,
      /^\s*(?:from\s+[\w.]+\s+import\s+|import\s+[\w.]+\s*$)/m,
      /^\s*(?:if|elif|for|while|class|with|try|except)\b[^\n]*:\s*$/m,
      /\b(?:print|input)\s*\(/,
    ],
    cpp: [
      /^\s*#\s*include\s*[<"]/m,
      /\b(?:std::|using\s+namespace\s+std\s*;)/,
      /\b(?:int|signed|auto)\s+main\s*\(/,
      /\b(?:cin\s*>>|cout\s*<<|vector\s*<)/,
    ],
    golang: [
      /^\s*package\s+\w+/m,
      /^\s*func\s+(?:\([^\n]*\)\s*)?\w+\s*\(/m,
      /\bfmt\.(?:Print|Scan)\w*\s*\(/,
      /\b(?:\w+\s*:=|make\s*\(\s*\[\])/,
    ],
  };
  const ranked = (Object.keys(signals) as SubmissionLanguage[])
    .map((language) => ({language, score: signals[language].filter((signal) => signal.test(sample)).length}))
    .sort((a, b) => b.score - a.score);
  // A single distinctive declaration is enough; weak expressions need support.
  const best = ranked[0];
  const strongSignals = {python: signals.python[0], cpp: signals.cpp[0], golang: signals.golang[1]};
  const strong = strongSignals[best.language].test(sample);
  return (best.score >= 2 || strong) && best.score > ranked[1].score ? best.language : null;
};
