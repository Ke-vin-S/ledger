import { RefreshCw, TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export type ErrorStateProps = {
  title?: string;
  description?: React.ReactNode;
  onRetry?: () => void;
  retryLabel?: string;
  className?: string;
};

export function ErrorState({
  title = "Something went wrong",
  onRetry,
  description = "We couldn't complete this request. Please check your connection and try again.",
  retryLabel = "Try again",
  className,
}: ErrorStateProps) {
  return (
    <div
      role="alert"
      className={cn(
        "flex min-h-64 w-full flex-col items-center justify-center rounded-xl border border-destructive/30 bg-card px-6 py-12 text-center",
        className,
      )}
    >
      <div
        className="mb-4 inline-flex h-12 w-12 items-center justify-center rounded-lg bg-destructive/10 text-destructive"
        aria-hidden="true"
      >
        <TriangleAlert className="h-6 w-6" />
      </div>
      <h2 className="text-base font-semibold text-foreground">{title}</h2>
      <p className="mt-2 max-w-md text-sm leading-relaxed text-muted-foreground">
        {description}
      </p>
      {onRetry ? (
        <Button
          type="button"
          variant="outline"
          className="mt-6"
          onClick={onRetry}
        >
          <RefreshCw className="h-4 w-4" aria-hidden="true" />
          {retryLabel}
        </Button>
      ) : null}
    </div>
  );
}
