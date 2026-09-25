"use client";

import type { UseQueryResult } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { Skeleton } from "@/components/shared/Skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { cn } from "@/lib/utils";

export type QueryBoundaryState<TData, TError extends Error = Error> = Pick<
  UseQueryResult<TData, TError>,
  "data" | "error" | "isLoading" | "isPending" | "refetch"
>;

export type QueryBoundaryContent<TData> = (data: TData) => ReactNode;

export type QueryBoundaryProps<
  TData,
  TError extends Error = Error,
> = QueryBoundaryState<TData, TError> & {
  /** Define emptiness explicitly so an absent error can never look like valid empty data. */
  isEmpty: (data: TData) => boolean;
  children?: ReactNode | QueryBoundaryContent<TData>;
  className?: string;
  loadingLabel?: string;
  errorMessage?: string;
  emptyMessage?: string;
  retryLabel?: string;
};

const loadingClassName =
  "flex min-h-40 flex-col items-center justify-center rounded-xl border border-border bg-card px-6 py-10";

export function QueryBoundary<TData, TError extends Error = Error>({
  data,
  error,
  isLoading,
  isPending,
  refetch,
  isEmpty,
  children,
  className,
  loadingLabel = "Loading…",
  errorMessage = "We couldn't load this right now. Please try again.",
  emptyMessage = "There's nothing to show here yet.",
  retryLabel = "Try again",
}: QueryBoundaryProps<TData, TError>) {
  if (error) {
    return (
      <ErrorState
        title="Unable to load"
        description={errorMessage}
        onRetry={() => void refetch()}
        retryLabel={retryLabel}
        className={className}
      />
    );
  }

  if (isPending || isLoading) {
    return (
      <div
        role="status"
        aria-live="polite"
        aria-busy="true"
        className={cn(loadingClassName, className)}
      >
        <span className="sr-only">{loadingLabel}</span>
        <div className="w-full max-w-xs space-y-3" aria-hidden="true">
          <Skeleton className="h-4 w-40" />
          <Skeleton className="h-3 w-full" />
          <Skeleton className="h-3 w-4/5" />
        </div>
      </div>
    );
  }

  if (data === undefined) {
    return (
      <ErrorState
        title="Unable to load"
        onRetry={() => void refetch()}
        description={errorMessage}
        retryLabel={retryLabel}
        className={className}
      />
    );
  }

  if (isEmpty(data)) {
    return (
      <EmptyState
        title="Nothing here yet"
        description={emptyMessage}
        className={className}
      />
    );
  }

  if (typeof children === "function") {
    return children(data);
  }

  return children;
}
