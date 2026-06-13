import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { CurrencyAmount } from "./CurrencyAmount";

describe("CurrencyAmount", () => {
  it("formats a positive amount", () => {
    render(<CurrencyAmount amount={150000} />);
    expect(screen.getByText("LKR 1,500.00")).toBeInTheDocument();
  });

  it("prefixes '+' and applies the positive colour when signed and positive", () => {
    render(<CurrencyAmount amount={150000} signed />);
    const el = screen.getByText("+", { exact: false });
    expect(el.textContent).toContain("+LKR 1,500.00");
    expect(el.className).toContain("--positive");
  });

  it("shows the absolute value (no minus) and negative colour when signed and negative", () => {
    render(<CurrencyAmount amount={-2500} signed />);
    const el = screen.getByText("LKR 25.00");
    expect(el.textContent).toBe("LKR 25.00");
    expect(el.className).toContain("--negative");
  });

  it("does not colour or sign when not signed", () => {
    render(<CurrencyAmount amount={-2500} />);
    const el = screen.getByText("LKR 25.00");
    expect(el.className).not.toContain("--negative");
  });

  it("respects a custom currency", () => {
    render(<CurrencyAmount amount={150000} currency="USD" />);
    expect(screen.getByText("USD 1,500.00")).toBeInTheDocument();
  });
});
