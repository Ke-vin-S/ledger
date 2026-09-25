"use client";

import { useEffect, useMemo, useState } from "react";
import { Avatar } from "@/components/shared/Avatar";
import { AmountInput } from "@/components/expense/AmountInput";
import { Input } from "@/components/ui/input";
import { formatAmount } from "@/lib/utils";
import { cn } from "@/lib/utils";
import { CheckCircle2, AlertCircle } from "lucide-react";
import type { PickedMember } from "@/types/team.types";
import type { SplitMethod, SplitEntry } from "@/types/expense.types";

export type { SplitMethod, SplitEntry };

type Props = {
  participants: PickedMember[];
  total: number;
  currency: string;
  method: SplitMethod;
  value: SplitEntry[];
  onChange: (entries: SplitEntry[]) => void;
  onValidityChange: (valid: boolean, message?: string) => void;
};

function computeEqualShare(total: number, n: number): number {
  if (n === 0) return 0;
  return Math.floor(total / n);
}

function allocateProportionally(
  total: number,
  units: number[],
  rounding: "round" | "floor",
): number[] {
  const totalUnits = units.reduce((sum, unit) => sum + Math.max(0, unit), 0);
  let assigned = 0;
  return units.map((unit, index) => {
    if (index === units.length - 1) return total - assigned;
    const amount =
      rounding === "round"
        ? Math.round((Math.max(0, unit) / totalUnits) * total)
        : Math.floor((Math.max(0, unit) / totalUnits) * total);
    assigned += amount;
    return amount;
  });
}

export function SplitBuilder({
  participants,
  total,
  currency,
  method,
  onChange,
  onValidityChange,
}: Props) {
  const [inputs, setInputs] = useState<Record<string, number>>({});
  const validity = useMemo(() => {
    if (participants.length === 0)
      return { valid: false, message: "Select at least one participant." };
    if (method === "equal")
      return {
        valid: total > 0,
        message: total > 0 ? "Split equally." : "Enter an amount.",
      };
    if (method === "exact") {
      const assigned = Object.values(inputs).reduce(
        (sum, value) => sum + (value || 0),
        0,
      );
      const remaining = total - assigned;
      return {
        valid: remaining === 0,
        message:
          remaining === 0
            ? "Splits sum to total."
            : `${remaining > 0 ? "+" : ""}${formatAmount(remaining, currency)} remaining`,
      };
    }
    if (method === "percentage") {
      const values = participants.map(
        (participant) => inputs[participant.id] ?? 0,
      );
      const sum = values.reduce(
        (totalPercent, value) => totalPercent + value,
        0,
      );
      const inRange = values.every((value) => value >= 0 && value <= 100);
      const valid = inRange && Math.abs(sum - 100) <= 0.01;
      return {
        valid,
        message: valid
          ? "Percentages sum to 100%."
          : !inRange
            ? "Each percentage must be between 0 and 100."
            : `${sum}% / 100% — ${100 - sum}% remaining`,
      };
    }
    const values = participants.map(
      (participant) => inputs[participant.id] ?? 0,
    );
    const valid = values.every((value) => value > 0);
    return {
      valid,
      message: valid
        ? "Proportional by weight."
        : "Every share must be greater than zero.",
    };
  }, [currency, inputs, method, participants, total]);

  useEffect(() => {
    onValidityChange(validity.valid, validity.message);
  }, [onValidityChange, validity.message, validity.valid]);

  // Reset inputs whenever participants or method change
  useEffect(() => {
    if (method === "equal") {
      onChange(participants.map((p) => ({ user_id: p.id })));
    } else if (method === "percentage") {
      const evenPct =
        participants.length > 0 ? Math.floor(100 / participants.length) : 0;
      const init: Record<string, number> = {};
      participants.forEach((p) => {
        init[p.id] = evenPct;
      });
      setInputs(init);
      onChange(
        participants.map((p) => ({ user_id: p.id, share_units: evenPct })),
      );
    } else if (method === "shares") {
      const init: Record<string, number> = {};
      participants.forEach((p) => {
        init[p.id] = 1;
      });
      setInputs(init);
      onChange(participants.map((p) => ({ user_id: p.id, share_units: 1 })));
    } else if (method === "exact") {
      const init: Record<string, number> = {};
      participants.forEach((p) => {
        init[p.id] = 0;
      });
      setInputs(init);
      onChange(participants.map((p) => ({ user_id: p.id, share_amount: 0 })));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [method, participants.map((p) => p.id).join(",")]);

  if (participants.length === 0) {
    return (
      <p className="text-xs text-muted-foreground italic">
        Select participants above to configure splits.
      </p>
    );
  }

  // ── Equal ──────────────────────────────────────────────────────────────────
  if (method === "equal") {
    const share = computeEqualShare(total, participants.length);
    const remainder = total - share * participants.length;
    return (
      <div className="space-y-1.5">
        {participants.map((p, i) => (
          <div key={p.id} className="flex items-center gap-2 text-sm">
            <Avatar
              name={p.name}
              size="sm"
              className="h-5 w-5 text-xs flex-shrink-0"
            />
            <span className="flex-1 truncate text-xs">{p.name}</span>
            <span className="text-xs font-mono text-muted-foreground">
              {formatAmount(share + (i === 0 ? remainder : 0), currency)}
            </span>
          </div>
        ))}
        <SplitValidation
          valid={validity.valid}
          label={validity.message ?? "Invalid split"}
        />
      </div>
    );
  }

  // ── Exact ──────────────────────────────────────────────────────────────────
  if (method === "exact") {
    const remaining =
      total -
      Object.values(inputs).reduce((sum, value) => sum + (value || 0), 0);
    const valid = remaining === 0;

    function setExact(uid: string, v: number) {
      const next = { ...inputs, [uid]: v };
      setInputs(next);
      onChange(
        participants.map((p) => ({
          user_id: p.id,
          share_amount: next[p.id] ?? 0,
        })),
      );
    }

    return (
      <div className="space-y-2">
        {participants.map((p) => (
          <div key={p.id} className="flex items-center gap-2">
            <Avatar
              name={p.name}
              size="sm"
              className="h-5 w-5 text-xs flex-shrink-0"
            />
            <span className="flex-1 truncate text-xs">{p.name}</span>
            <div className="w-32">
              <AmountInput
                aria-label={`Exact share for ${p.name}`}
                value={inputs[p.id] ?? 0}
                currency={currency}
                onChange={(v) => setExact(p.id, v)}
              />
            </div>
          </div>
        ))}
        <SplitValidation
          valid={valid}
          label={validity.message ?? "Invalid split"}
        />
      </div>
    );
  }

  // ── Percentage ─────────────────────────────────────────────────────────────
  if (method === "percentage") {
    const amounts = allocateProportionally(
      total,
      participants.map((participant) => inputs[participant.id] ?? 0),
      "round",
    );

    function setPct(uid: string, v: number) {
      const next = { ...inputs, [uid]: v };
      setInputs(next);
      onChange(
        participants.map((p) => ({
          user_id: p.id,
          share_units: next[p.id] ?? 0,
        })),
      );
    }

    return (
      <div className="space-y-2">
        {participants.map((p, index) => {
          const pct = inputs[p.id] ?? 0;
          const computed = amounts[index] ?? 0;
          return (
            <div key={p.id} className="flex items-center gap-2">
              <Avatar
                name={p.name}
                size="sm"
                className="h-5 w-5 text-xs flex-shrink-0"
              />
              <span className="flex-1 truncate text-xs">{p.name}</span>
              <div className="flex items-center gap-1.5 w-36">
                <Input
                  aria-label={`Percentage share for ${p.name}`}
                  type="number"
                  min={0}
                  max={100}
                  value={pct}
                  onChange={(event) => setPct(p.id, Number(event.target.value))}
                  className="h-9 w-14 px-2 text-right text-xs"
                />
                <span className="text-xs text-muted-foreground">%</span>
                <span className="text-xs font-mono text-muted-foreground w-16 text-right truncate">
                  {formatAmount(computed, currency)}
                </span>
              </div>
            </div>
          );
        })}
        <SplitValidation
          valid={validity.valid}
          label={validity.message ?? "Invalid split"}
        />
      </div>
    );
  }

  // ── Shares / Weights ───────────────────────────────────────────────────────
  if (method === "shares") {
    const amounts = allocateProportionally(
      total,
      participants.map((participant) => inputs[participant.id] ?? 0),
      "floor",
    );

    function setShares(uid: string, v: number) {
      const next = { ...inputs, [uid]: v };
      setInputs(next);
      onChange(
        participants.map((p) => ({
          user_id: p.id,
          share_units: next[p.id] ?? 1,
        })),
      );
    }

    return (
      <div className="space-y-2">
        {participants.map((p, index) => {
          const units = inputs[p.id] ?? 1;
          const computed = amounts[index] ?? 0;
          return (
            <div key={p.id} className="flex items-center gap-2">
              <Avatar
                name={p.name}
                size="sm"
                className="h-5 w-5 text-xs flex-shrink-0"
              />
              <span className="flex-1 truncate text-xs">{p.name}</span>
              <div className="flex items-center gap-1.5 w-36">
                <Input
                  aria-label={`Share units for ${p.name}`}
                  type="number"
                  min={1}
                  value={units}
                  onChange={(event) =>
                    setShares(p.id, Math.max(1, Number(event.target.value)))
                  }
                  className="h-9 w-14 px-2 text-right text-xs"
                />
                <span className="text-xs text-muted-foreground">×</span>
                <span className="text-xs font-mono text-muted-foreground w-16 text-right truncate">
                  {formatAmount(computed, currency)}
                </span>
              </div>
            </div>
          );
        })}
        <SplitValidation
          valid={validity.valid}
          label={validity.message ?? "Invalid split"}
        />
      </div>
    );
  }

  return null;
}

function SplitValidation({ valid, label }: { valid: boolean; label: string }) {
  return (
    <div
      role={valid ? "status" : "alert"}
      className={cn(
        "flex items-center gap-1 pt-1 text-xs",
        valid ? "text-positive" : "text-destructive",
      )}
    >
      {valid ? (
        <CheckCircle2 className="h-3.5 w-3.5" aria-hidden="true" />
      ) : (
        <AlertCircle className="h-3.5 w-3.5" aria-hidden="true" />
      )}
      <span>{label}</span>
    </div>
  );
}
