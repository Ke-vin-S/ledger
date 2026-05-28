"use client";

import * as Dialog from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { cn } from "@/lib/utils";

export const Sheet = Dialog.Root;
export const SheetTrigger = Dialog.Trigger;
export const SheetClose = Dialog.Close;

type SheetContentProps = {
  children: React.ReactNode;
  className?: string;
  title: string;
  description?: string;
};

export function SheetContent({ children, className, title, description }: SheetContentProps) {
  return (
    <Dialog.Portal>
      <Dialog.Overlay className="fixed inset-0 z-40 bg-black/50 backdrop-blur-sm data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
      <Dialog.Content
        className={cn(
          "fixed right-0 top-0 z-50 h-full w-full max-w-md bg-[hsl(var(--card))] shadow-xl flex flex-col",
          "data-[state=open]:animate-in data-[state=closed]:animate-out",
          "data-[state=closed]:slide-out-to-right data-[state=open]:slide-in-from-right",
          "duration-200",
          className,
        )}
      >
        <div className="flex items-center justify-between border-b px-6 py-4">
          <div>
            <Dialog.Title className="text-base font-semibold">{title}</Dialog.Title>
            {description && (
              <Dialog.Description className="text-xs text-[hsl(var(--muted-foreground))] mt-0.5">
                {description}
              </Dialog.Description>
            )}
          </div>
          <Dialog.Close className="rounded-md p-1.5 hover:bg-[hsl(var(--muted))] transition-colors">
            <X className="h-4 w-4" />
            <span className="sr-only">Close</span>
          </Dialog.Close>
        </div>
        <div className="flex-1 overflow-y-auto px-6 py-5">{children}</div>
      </Dialog.Content>
    </Dialog.Portal>
  );
}
