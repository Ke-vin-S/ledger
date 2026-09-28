import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { SplitBuilder } from "./SplitBuilder";
import type { PickedMember } from "@/types/team.types";

const participants: PickedMember[] = [
  { id: "u1", name: "Alice" },
  { id: "u2", name: "Bob" },
];

describe("SplitBuilder — equal", () => {
  it("gives the remainder to the first participant", () => {
    // 3001 minor units / 2 = 1500 floor, remainder 1 → first gets 1501, second 1500.
    render(
      <SplitBuilder
        participants={participants}
        total={3001}
        currency="LKR"
        method="equal"
        value={[]}
        onChange={() => {}}
        onValidityChange={() => {}}
      />,
    );
    expect(screen.getByText("LKR 15.01")).toBeInTheDocument();
    expect(screen.getByText("LKR 15.00")).toBeInTheDocument();
  });

  it("splits evenly with no remainder", () => {
    render(
      <SplitBuilder
        participants={participants}
        total={3000}
        currency="LKR"
        method="equal"
        value={[]}
        onChange={() => {}}
        onValidityChange={() => {}}
      />,
    );
    expect(screen.getAllByText("LKR 15.00")).toHaveLength(2);
  });

  it("emits one entry per participant via onChange", () => {
    const onChange = vi.fn();
    render(
      <SplitBuilder
        participants={participants}
        total={3000}
        currency="LKR"
        method="equal"
        value={[]}
        onChange={onChange}
        onValidityChange={() => {}}
      />,
    );
    expect(onChange).toHaveBeenCalledWith([{ user_id: "u1" }, { user_id: "u2" }]);
  });
});

describe("SplitBuilder — percentage", () => {
  it("initialises even percentages and previews the computed amount", () => {
    const onChange = vi.fn();
    render(
      <SplitBuilder
        participants={participants}
        total={10000}
        currency="LKR"
        method="percentage"
        value={[]}
        onChange={onChange}
        onValidityChange={() => {}}
      />,
    );
    // Even split: 50% each → 50.00 of 100.00.
    expect(screen.getAllByText("LKR 50.00").length).toBeGreaterThanOrEqual(2);
    expect(onChange).toHaveBeenCalledWith([
      { user_id: "u1", share_units: 50 },
      { user_id: "u2", share_units: 50 },
    ]);
  });
});

describe("SplitBuilder — exact", () => {
  it("reports an invalid total to the submit owner", () => {
    const onValidityChange = vi.fn();
    render(
      <SplitBuilder
        participants={participants}
        total={3000}
        currency="LKR"
        method="exact"
        value={[]}
        onChange={() => {}}
        onValidityChange={onValidityChange}
      />,
    );
    expect(onValidityChange).toHaveBeenLastCalledWith(false, "+LKR 30.00 remaining");
  });
});

describe("SplitBuilder — empty", () => {
  it("prompts to select participants when there are none", () => {
    render(
      <SplitBuilder
        participants={[]}
        total={1000}
        currency="LKR"
        method="equal"
        value={[]}
        onValidityChange={() => {}}
        onChange={() => {}}
      />,
    );
    expect(screen.getByText(/select participants/i)).toBeInTheDocument();
  });
});
