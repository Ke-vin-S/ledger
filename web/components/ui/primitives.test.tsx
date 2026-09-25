import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Dialog, DialogContent } from "./dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "./tabs";
import { Pagination } from "./pagination";

describe("UI primitives", () => {
  it("gives dialogs an accessible title and description", () => {
    render(
      <Dialog open>
        <DialogContent
          title="Record settlement"
          description="Confirm the amount with your teammate."
        >
          <button type="button">Confirm</button>
        </DialogContent>
      </Dialog>,
    );

    expect(
      screen.getByRole("dialog", { name: "Record settlement" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Confirm the amount with your teammate."),
    ).toBeInTheDocument();
  });

  it("uses tab semantics and switches panels", async () => {
    const user = userEvent.setup();
    render(
      <Tabs defaultValue="lent">
        <TabsList aria-label="Loan views">
          <TabsTrigger value="lent">Lent</TabsTrigger>
          <TabsTrigger value="borrowed">Borrowed</TabsTrigger>
        </TabsList>
        <TabsContent value="lent">Lent panel</TabsContent>
        <TabsContent value="borrowed">Borrowed panel</TabsContent>
      </Tabs>,
    );

    expect(screen.getByRole("tab", { name: "Lent" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByText("Lent panel")).toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "Borrowed" }));
    expect(screen.getByRole("tab", { name: "Borrowed" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByText("Borrowed panel")).toBeInTheDocument();
  });

  it("disables unavailable pagination directions", () => {
    const onPageChange = vi.fn();
    render(<Pagination page={1} totalPages={2} onPageChange={onPageChange} />);

    expect(
      screen.getByRole("button", { name: "Previous page" }),
    ).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Next page" }));
    expect(onPageChange).toHaveBeenCalledWith(2);
  });
});
