import * as React from "react";
import { cn } from "@/lib/utils";

export type EmptyStateProps = {
  icon?: React.ReactNode;
  title: string;
  description?: React.ReactNode;
  action?: React.ReactNode;
  className?: string;
};

export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  const titleId = React.useId();
  const descriptionId = React.useId();

  return (
    <section
      aria-labelledby={titleId}
      aria-describedby={description ? descriptionId : undefined}
      className={cn(
        "flex min-h-64 w-full flex-col items-center justify-center rounded-xl border border-dashed border-border bg-card px-6 py-12 text-center",
        className,
      )}
    >
      {icon ? (
        <div
          className="mb-4 inline-flex h-12 w-12 items-center justify-center rounded-lg bg-muted text-muted-foreground [&_svg]:h-6 [&_svg]:w-6"
          aria-hidden="true"
        >
          {icon}
        </div>
      ) : null}
      <h2 id={titleId} className="text-base font-semibold text-foreground">
        {title}
      </h2>
      {description ? (
        <p
          id={descriptionId}
          className="mt-2 max-w-md text-sm leading-relaxed text-muted-foreground"
        >
          {description}
        </p>
      ) : null}
      {action ? <div className="mt-6">{action}</div> : null}
    </section>
  );
}
