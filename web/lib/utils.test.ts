import { describe, it, expect } from "vitest";
import { formatAmount, parseAmount, formatDate, cn } from "./utils";

describe("formatAmount", () => {
  it("formats minor units as ISO code + comma-separated 2dp", () => {
    expect(formatAmount(150000)).toBe("LKR 1,500.00");
  });

  it("formats zero", () => {
    expect(formatAmount(0)).toBe("LKR 0.00");
  });

  it("handles sub-rupee amounts", () => {
    expect(formatAmount(50)).toBe("LKR 0.50");
  });

  it("formats large amounts with grouping", () => {
    expect(formatAmount(123456789)).toBe("LKR 1,234,567.89");
  });

  it("respects a custom currency code", () => {
    expect(formatAmount(150000, "USD")).toBe("USD 1,500.00");
  });

  it("formats negative minor units (sign preserved by toLocaleString)", () => {
    expect(formatAmount(-2500)).toBe("LKR -25.00");
  });
});

describe("parseAmount", () => {
  it("parses a plain decimal string to minor units", () => {
    expect(parseAmount("1500.50")).toBe(150050);
  });

  it("strips currency symbols and grouping", () => {
    expect(parseAmount("LKR 1,500.00")).toBe(150000);
  });

  it("parses integers (no decimal point) to minor units", () => {
    expect(parseAmount("42")).toBe(4200);
  });

  it("rounds to the nearest minor unit", () => {
    expect(parseAmount("10.005")).toBe(1001);
  });

  it("returns NaN for empty input", () => {
    expect(parseAmount("")).toBeNaN();
  });

  it("returns NaN for non-numeric input", () => {
    expect(parseAmount("abc")).toBeNaN();
  });

  it("round-trips with formatAmount", () => {
    expect(parseAmount(formatAmount(150000))).toBe(150000);
  });
});

describe("formatDate", () => {
  it("formats an ISO date as 'Mon D, YYYY'", () => {
    expect(formatDate("2026-01-15T00:00:00Z")).toMatch(/Jan 1[45], 2026/);
  });
});

describe("cn", () => {
  it("merges class names and de-dupes conflicting tailwind utilities", () => {
    expect(cn("px-2", "px-4")).toBe("px-4");
  });

  it("drops falsy values", () => {
    expect(cn("a", false && "b", undefined, "c")).toBe("a c");
  });
});
