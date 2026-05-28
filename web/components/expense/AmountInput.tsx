"use client";

import { useState } from "react";
import { Input } from "@/components/ui/input";
import { parseAmount } from "@/lib/utils";

type Props = {
  value: number;
  onChange: (minorUnits: number) => void;
  currency?: string;
  placeholder?: string;
  className?: string;
};

export function AmountInput({
  value,
  onChange,
  currency = "LKR",
  placeholder = "0.00",
  className,
}: Props) {
  const [display, setDisplay] = useState(
    value > 0 ? (value / 100).toFixed(2) : "",
  );

  function handleChange(e: React.ChangeEvent<HTMLInputElement>) {
    const raw = e.target.value;
    setDisplay(raw);
    const minor = parseAmount(raw);
    if (!isNaN(minor)) onChange(minor);
  }

  function handleBlur() {
    const minor = parseAmount(display);
    if (!isNaN(minor) && minor > 0) {
      setDisplay((minor / 100).toFixed(2));
    }
  }

  return (
    <div className="flex items-center gap-2">
      <span className="text-sm text-[hsl(var(--muted-foreground))] flex-shrink-0">
        {currency}
      </span>
      <Input
        type="text"
        inputMode="decimal"
        value={display}
        onChange={handleChange}
        onBlur={handleBlur}
        placeholder={placeholder}
        className={className}
      />
    </div>
  );
}
