import { cn } from "@/lib/utils";

type Props = {
  name: string;
  src?: string;
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

export function Avatar({ name, src, size = "md", className }: Props) {
  const base = cn(
    "rounded-full flex items-center justify-center font-medium flex-shrink-0 overflow-hidden",
    sizeClasses[size],
    className,
  );

  if (src) {
    return (
      <div className={base} title={name}>
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={src}
          alt={name}
          className="h-full w-full object-cover"
          onError={(e) => {
            // Fall back to initials if the image fails to load.
            const target = e.currentTarget;
            target.style.display = "none";
            target.parentElement!.textContent = initials(name);
          }}
        />
      </div>
    );
  }

  return (
    <div
      className={cn(base, "bg-[hsl(var(--muted))] text-[hsl(var(--muted-foreground))]")}
      title={name}
    >
      {initials(name)}
    </div>
  );
}
