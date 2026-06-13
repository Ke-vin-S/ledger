import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { AmountInput } from "./AmountInput";

describe("AmountInput", () => {
  it("renders the currency label and an empty field for zero value", () => {
    render(<AmountInput value={0} onChange={() => {}} />);
    expect(screen.getByText("LKR")).toBeInTheDocument();
    expect(screen.getByRole("textbox")).toHaveValue("");
  });

  it("pre-fills the display from an existing minor-units value", () => {
    render(<AmountInput value={150050} onChange={() => {}} />);
    expect(screen.getByRole("textbox")).toHaveValue("1500.50");
  });

  it("emits minor units as the user types", () => {
    const onChange = vi.fn();
    render(<AmountInput value={0} onChange={onChange} />);

    fireEvent.change(screen.getByRole("textbox"), { target: { value: "12.34" } });
    expect(onChange).toHaveBeenLastCalledWith(1234);
  });

  it("normalises the display to 2 decimals on blur", () => {
    render(<AmountInput value={0} onChange={() => {}} />);
    const input = screen.getByRole("textbox");

    fireEvent.change(input, { target: { value: "12.5" } });
    fireEvent.blur(input);
    expect(input).toHaveValue("12.50");
  });

  it("does not emit when the input is not a number", () => {
    const onChange = vi.fn();
    render(<AmountInput value={0} onChange={onChange} />);

    fireEvent.change(screen.getByRole("textbox"), { target: { value: "abc" } });
    expect(onChange).not.toHaveBeenCalled();
  });
});
