import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryBoundary, type QueryBoundaryState } from "./query-boundary";

function state<T>(
  overrides: Partial<QueryBoundaryState<T>> = {},
): QueryBoundaryState<T> {
  return {
    data: undefined,
    error: null,
    isLoading: false,
    isPending: false,
    refetch: vi.fn(async () => ({}) as never),
    ...overrides,
  };
}

describe("QueryBoundary", () => {
  it("renders a persistent error state instead of an empty state", () => {
    render(
      <QueryBoundary
        {...state<number>({ error: new Error("network down") })}
        isEmpty={() => false}
        errorMessage="Could not load balances."
      >
        <div>private data</div>
      </QueryBoundary>,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Could not load balances.",
    );
    expect(screen.queryByText("private data")).not.toBeInTheDocument();
  });

  it("renders loading before it evaluates data", () => {
    render(
      <QueryBoundary
        {...state<number>({ isPending: true })}
        isEmpty={() => false}
      >
        <div>private data</div>
      </QueryBoundary>,
    );

    expect(screen.getByRole("status")).toHaveAttribute("aria-busy", "true");
    expect(screen.queryByText("private data")).not.toBeInTheDocument();
  });

  it("renders an explicit empty state without invoking children", () => {
    render(
      <QueryBoundary
        {...state<string[]>({ data: [] })}
        isEmpty={(items) => items.length === 0}
        emptyMessage="No activity yet."
      >
        <div>private data</div>
      </QueryBoundary>,
    );

    expect(screen.getByText("No activity yet.")).toBeInTheDocument();
    expect(screen.queryByText("private data")).not.toBeInTheDocument();
  });

  it("renders children for successful non-empty data", () => {
    render(
      <QueryBoundary
        {...state<string[]>({ data: ["one"] })}
        isEmpty={(items) => items.length === 0}
      >
        {(items) => <div>{items.join(",")}</div>}
      </QueryBoundary>,
    );

    expect(screen.getByText("one")).toBeInTheDocument();
  });
});
