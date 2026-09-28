import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export type PaginationProps = {
  page: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  previousLabel?: string;
  nextLabel?: string;
  label?: string;
  className?: string;
};

export function Pagination({
  page,
  totalPages,
  onPageChange,
  previousLabel = "Previous page",
  nextLabel = "Next page",
  label = "Pagination",
  className,
}: PaginationProps) {
  const pageCount = Math.max(1, totalPages);
  const isFirstPage = page <= 1;
  const isLastPage = page >= pageCount;

  return (
    <nav
      aria-label={label}
      className={cn("flex items-center justify-between gap-4", className)}
    >
      <Button
        type="button"
        variant="outline"
        disabled={isFirstPage}
        aria-label={previousLabel}
        onClick={() => onPageChange(page - 1)}
      >
        <ChevronLeft className="h-4 w-4" aria-hidden="true" />
        <span className="hidden sm:inline">Previous</span>
        <span className="sr-only sm:hidden">{previousLabel}</span>
      </Button>
      <p className="text-sm font-medium text-muted-foreground" aria-live="polite">
        Page {Math.min(Math.max(page, 1), pageCount)} of {pageCount}
      </p>
      <Button
        type="button"
        variant="outline"
        disabled={isLastPage}
        aria-label={nextLabel}
        onClick={() => onPageChange(page + 1)}
      >
        <span className="hidden sm:inline">Next</span>
        <span className="sr-only sm:hidden">{nextLabel}</span>
        <ChevronRight className="h-4 w-4" aria-hidden="true" />
      </Button>
    </nav>
  );
}
