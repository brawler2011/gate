import {describe, it, expect} from "bun:test";

import {numberToLetters, lettersToNumber} from "../lib/lib";

describe("Problem position and letter conversion suite", () => {
  describe("numberToLetters", () => {
    it("converts single-digit numbers to letters correctly", () => {
      expect(numberToLetters(1)).toBe("A");
      expect(numberToLetters(2)).toBe("B");
      expect(numberToLetters(26)).toBe("Z");
    });

    it("converts multi-digit numbers to letters correctly", () => {
      expect(numberToLetters(27)).toBe("AA");
      expect(numberToLetters(28)).toBe("AB");
      expect(numberToLetters(52)).toBe("AZ");
      expect(numberToLetters(53)).toBe("BA");
      expect(numberToLetters(702)).toBe("ZZ");
      expect(numberToLetters(703)).toBe("AAA");
    });

    it("handles 0, negative numbers, null and undefined", () => {
      expect(numberToLetters(0)).toBe("?");
      expect(numberToLetters(-1)).toBe("?");
      expect(numberToLetters(null)).toBe("?");
      expect(numberToLetters(undefined)).toBe("?");
    });
  });

  describe("lettersToNumber", () => {
    it("converts single letters to numbers correctly", () => {
      expect(lettersToNumber("A")).toBe(1);
      expect(lettersToNumber("B")).toBe(2);
      expect(lettersToNumber("Z")).toBe(26);
    });

    it("converts multi-letter strings to numbers correctly", () => {
      expect(lettersToNumber("AA")).toBe(27);
      expect(lettersToNumber("AB")).toBe(28);
      expect(lettersToNumber("AZ")).toBe(52);
      expect(lettersToNumber("BA")).toBe(53);
      expect(lettersToNumber("ZZ")).toBe(702);
      expect(lettersToNumber("AAA")).toBe(703);
    });

    it("strictly rejects lowercase letters", () => {
      expect(lettersToNumber("a")).toBe(0);
      expect(lettersToNumber("b")).toBe(0);
      expect(lettersToNumber("aa")).toBe(0);
      expect(lettersToNumber("Ab")).toBe(0);
    });

    it("rejects non-alphabetic, empty, null and undefined inputs", () => {
      expect(lettersToNumber("")).toBe(0);
      expect(lettersToNumber("1")).toBe(0);
      expect(lettersToNumber("A1")).toBe(0);
      expect(lettersToNumber("A-B")).toBe(0);
      expect(lettersToNumber("UUID-1234")).toBe(0);
      expect(lettersToNumber(null)).toBe(0);
      expect(lettersToNumber(undefined)).toBe(0);
    });

    it("is bidirectional with numberToLetters for all positive integers up to 1000", () => {
      for (let i = 1; i <= 1000; i++) {
        const letter = numberToLetters(i);
        const num = lettersToNumber(letter);
        expect(num).toBe(i);
      }
    });
  });
});
