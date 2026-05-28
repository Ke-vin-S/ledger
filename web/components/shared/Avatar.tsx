import { cn } from "@/lib/utils";

type Props = {
  name: string;
  size?: "sm" | "md" | "lg";
  className?: string;
};

function initials(name: string): string {
  return name
    .split(" ")
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? "")
    .join("");
}

const sizeClasses = {
  sm: "h-7 w-7 text-xs",
  md: "h-9 w-9 text-sm",
  lg: "h-12 w-12 text-base",
};

export function Avatar({ name, size = "md", className }: Props) {
  return (
    <div
      className={cn(
        "rounded-full bg-[hsl(var(--muted))] text-[hsl(var(--muted-foreground))] flex items-center justify-center font-medium flex-shrink-0",
        sizeClasses[size],
        className,
      )}
      title={name}
    >
      {initials(name)}
    </div>
  );
}
