import * as React from "react";
import {
  CheckCircle2,
  Info,
  TriangleAlert,
  XCircle,
  type LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";

const alertVariants = {
  info: {
    icon: Info,
    className: "border-border bg-muted/50 text-foreground",
    iconClassName: "text-foreground",
  },
  success: {
    icon: CheckCircle2,
    className: "border-positive/35 bg-positive/10 text-foreground",
    iconClassName: "text-positive",
  },
  warning: {
    icon: TriangleAlert,
    className: "border-pending/35 bg-pending/10 text-foreground",
    iconClassName: "text-pending",
  },
  destructive: {
    icon: XCircle,
    className: "border-destructive/35 bg-destructive/10 text-foreground",
    iconClassName: "text-destructive",
  },
} as const;

export type AlertVariant = keyof typeof alertVariants;

export type AlertProps = React.HTMLAttributes<HTMLDivElement> & {
  variant?: AlertVariant;
  icon?: LucideIcon | null;
};

export function Alert({
  className,
  variant = "info",
  icon,
  ...props
}: AlertProps) {
  const styles = alertVariants[variant];
  const Icon = icon === null ? null : (icon ?? styles.icon);

  return (
    <div
      role={
        variant === "destructive" || variant === "warning" ? "alert" : "status"
      }
      className={cn(
        "flex w-full gap-3 rounded-lg border p-4 text-sm",
        styles.className,
        className,
      )}
      {...props}
    >
      {Icon ? (
        <Icon
          className={cn(
            "mt-0.5 h-5 w-5 shrink-0",
            styles.iconClassName,
            icon && "text-foreground",
          )}
          aria-hidden="true"
        />
      ) : null}
      <div className="min-w-0 flex-1">{props.children}</div>
    </div>
  );
}

export function AlertTitle({
  className,
  ...props
}: React.HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h2 className={cn("mb-1 font-semibold leading-none", className)} {...props} />
  );
}

export function AlertDescription({
  className,
  ...props
}: React.HTMLAttributes<HTMLParagraphElement>) {
  return (
    <div className={cn("text-sm leading-relaxed text-muted-foreground", className)} {...props} />
  );
}
