"use client";

import * as React from "react";
import * as ToastPrimitive from "@radix-ui/react-toast";
import { cva, type VariantProps } from "class-variance-authority";
import { X } from "lucide-react";

const toastVariants = cva(
  "group pointer-events-auto relative flex w-full items-start gap-3 overflow-hidden rounded-lg border p-4 pr-10 shadow-lg transition-all data-[swipe=cancel]:translate-x-0 data-[swipe=end]:translate-x-[var(--radix-toast-swipe-end-x)] data-[swipe=move]:translate-x-[var(--radix-toast-swipe-move-x)] data-[swipe=move]:transition-none data-[state=closed]:animate-out data-[state=open]:animate-in data-[swipe=end]:animate-out",
  {
    variants: {
      variant: {
        default: "border-border bg-card text-card-foreground",
        destructive:
          "border-destructive/40 bg-destructive/10 text-foreground [&_[data-toast-title]]:text-destructive",
        success:
          "border-positive/40 bg-positive/10 text-foreground [&_[data-toast-title]]:text-positive",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
);

export type ToastVariant = NonNullable<
  VariantProps<typeof toastVariants>["variant"]
>;

export type ToastOptions = {
  title: React.ReactNode;
  description?: React.ReactNode;
  variant?: ToastVariant;
  duration?: number;
};

type ToastItem = ToastOptions & { id: string };

type ToastContextValue = {
  toast: (message: string | ToastOptions) => string;
  dismiss: (id: string) => void;
};

const ToastContext = React.createContext<ToastContextValue | null>(null);

export function useToast(): ToastContextValue {
  const context = React.useContext(ToastContext);
  if (!context) {
    throw new Error("useToast must be used within a ToastProvider");
  }
  return context;
}

export type ToastProviderProps = React.PropsWithChildren<{
  duration?: number;
}>;
function Toaster({
  items,
  onDismiss,
}: {
  items: ToastItem[];
  onDismiss: (id: string) => void;
}) {
  return items.map((item) => (
    <Toast
      key={item.id}
      {...item}
      open
      onOpenChange={(open) => {
        if (!open) onDismiss(item.id);
      }}
    />
  ));
}

export function ToastProvider({
  children,
  duration = 5000,
}: ToastProviderProps) {
  const [items, setItems] = React.useState<ToastItem[]>([]);
  const nextId = React.useRef(0);

  const dismiss = React.useCallback((id: string) => {
    setItems((current) => current.filter((item) => item.id !== id));
  }, []);

  const toast = React.useCallback((message: string | ToastOptions) => {
    const options = typeof message === "string" ? { title: message } : message;
    const id = `toast-${++nextId.current}`;
    setItems((current) => [...current, { ...options, id }]);
    return id;
  }, []);

  const context = React.useMemo(() => ({ toast, dismiss }), [dismiss, toast]);

  return (
    <ToastContext.Provider value={context}>
      <ToastPrimitive.Provider duration={duration} swipeDirection="right">
        {children}
        <Toaster items={items} onDismiss={dismiss} />
        <ToastPrimitive.Viewport
          className="fixed right-0 top-0 z-[100] flex max-h-screen w-full max-w-sm flex-col gap-3 p-4 sm:top-auto sm:bottom-0 sm:right-0"
          label="Notifications"
        />
      </ToastPrimitive.Provider>
    </ToastContext.Provider>
  );
}

export type ToastProps = Omit<
  React.ComponentPropsWithoutRef<typeof ToastPrimitive.Root>,
  "title"
> &
  ToastOptions;

export function Toast({
  title,
  description,
  variant = "default",
  duration,
  onOpenChange,
  ...props
}: ToastProps) {
  return (
    <ToastPrimitive.Root
      duration={duration}
      onOpenChange={onOpenChange}
      className={toastVariants({ variant })}
      aria-label={typeof title === "string" ? title : "Notification"}
      {...props}
    >
      <div className="min-w-0 flex-1 space-y-1">
        <ToastPrimitive.Title
          data-toast-title=""
          className="text-sm font-semibold leading-none"
        >
          {title}
        </ToastPrimitive.Title>
        {description ? (
          <ToastPrimitive.Description className="text-sm leading-relaxed text-muted-foreground">
            {description}
          </ToastPrimitive.Description>
        ) : null}
      </div>
      <ToastPrimitive.Close
        className="absolute right-2 top-2 inline-flex h-9 w-9 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        aria-label="Dismiss notification"
      >
        <X className="h-4 w-4" aria-hidden="true" />
      </ToastPrimitive.Close>
    </ToastPrimitive.Root>
  );
}
