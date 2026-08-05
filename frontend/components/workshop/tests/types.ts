export type TestItem = {
  id: string;
  ordinal: number;
  filename: string;
  outFilename: string;
  hasIn: boolean;
  hasOut: boolean;
  isSample: boolean;
  method: "manual" | "generated";
  generatorCommand?: string;
  subtaskNames: string[];
  validatorStatus?: {
    valid?: boolean;
    message?: string;
    error?: string;
  };
  solutionStatus?: {
    verdict?: string;
    time?: number;
    memory?: number;
    message?: string;
  };
};

export type SubtaskItem = {
  name: string;
  points: number;
  policy: "complete" | "each";
  dependencies: string[];
  testIds: string[];
};

export function parseOrdinalsFromRanges(input: string): number[] {
  const result = new Set<number>();
  const parts = input.split(",").map((s) => s.trim()).filter(Boolean);

  for (const part of parts) {
    if (part.includes("-")) {
      const [startStr, endStr] = part.split("-").map((s) => s.trim());
      const start = parseInt(startStr, 10);
      const end = parseInt(endStr, 10);
      if (!isNaN(start) && !isNaN(end)) {
        const from = Math.min(start, end);
        const to = Math.max(start, end);
        for (let i = from; i <= to; i++) {
          if (i > 0) {
            result.add(i);
          }
        }
      } else if (!isNaN(start) && (endStr === "" || isNaN(end))) {
        if (start > 0) {
          result.add(start);
        }
      }
    } else {
      const val = parseInt(part, 10);
      if (!isNaN(val) && val > 0) {
        result.add(val);
      }
    }
  }

  return Array.from(result).sort((a, b) => a - b);
}

export function formatOrdinalsToRanges(ordinals: number[]): string {
  if (!ordinals || ordinals.length === 0) {
    return "";
  }
  const sorted = Array.from(new Set(ordinals)).sort((a, b) => a - b);
  const ranges: string[] = [];

  let start = sorted[0];
  let prev = sorted[0];

  for (let i = 1; i < sorted.length; i++) {
    const curr = sorted[i];
    if (curr === prev + 1) {
      prev = curr;
    } else {
      if (start === prev) {
        ranges.push(`${start}`);
      } else {
        ranges.push(`${start}-${prev}`);
      }
      start = curr;
      prev = curr;
    }
  }

  if (start === prev) {
    ranges.push(`${start}`);
  } else {
    ranges.push(`${start}-${prev}`);
  }

  return ranges.join(", ");
}

export function formatPaddedOrdinal(n: number): string {
  return n < 10 ? `0${n}` : `${n}`;
}
