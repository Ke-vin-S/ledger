"use client";

import { useEffect, useState } from "react";
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
};

function computeEqualShare(total: number, n: number): number {
  if (n === 0) return 0;
  return Math.floor(total / n);
}

export function SplitBuilder({ participants, total, currency, method, onChange }: Props) {
  const [inputs, setInputs] = useState<Record<string, number>>({});

  // Reset inputs whenever participants or method change
  useEffect(() => {
    if (method === "equal") {
      onChange(participants.map((p) => ({ user_id: p.id })));
    } else if (method === "percentage") {
      const evenPct = participants.length > 0 ? Math.floor(100 / participants.length) : 0;
      const init: Record<string, number> = {};
      participants.forEach((p) => { init[p.id] = evenPct; });
      setInputs(init);
      onChange(participants.map((p) => ({ user_id: p.id, share_units: evenPct })));
    } else if (method === "shares") {
      const init: Record<string, number> = {};
      participants.forEach((p) => { init[p.id] = 1; });
      setInputs(init);
      onChange(participants.map((p) => ({ user_id: p.id, share_units: 1 })));
    } else if (method === "exact") {
      const init: Record<string, number> = {};
      participants.forEach((p) => { init[p.id] = 0; });
      setInputs(init);
      onChange(participants.map((p) => ({ user_id: p.id, share_amount: 0 })));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [method, participants.map((p) => p.id).join(",")]);

  if (participants.length === 0) {
    return (
      <p className="text-xs text-[hsl(var(--muted-foreground))] italic">
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
            <Avatar name={p.name} size="sm" className="h-5 w-5 text-[0.55rem] flex-shrink-0" />
            <span className="flex-1 truncate text-xs">{p.name}</span>
            <span className="text-xs font-mono text-[hsl(var(--muted-foreground))]">
              {formatAmount(share + (i === 0 ? remainder : 0), currency)}
            </span>
          </div>
        ))}
        <div className="flex items-center gap-1 pt-1 text-[hsl(var(--primary))] text-xs">
          <CheckCircle2 className="h-3.5 w-3.5" />
          <span>Split equally</span>
        </div>
      </div>
    );
  }

  // ── Exact ──────────────────────────────────────────────────────────────────
  if (method === "exact") {
    const assigned = Object.values(inputs).reduce((s, v) => s + (v || 0), 0);
    const remaining = total - assigned;
    const valid = remaining === 0;

    function setExact(uid: string, v: number) {
      const next = { ...inputs, [uid]: v };
      setInputs(next);
      onChange(participants.map((p) => ({ user_id: p.id, share_amount: next[p.id] ?? 0 })));
    }

    return (
      <div className="space-y-2">
        {participants.map((p) => (
          <div key={p.id} className="flex items-center gap-2">
            <Avatar name={p.name} size="sm" className="h-5 w-5 text-[0.55rem] flex-shrink-0" />
            <span className="flex-1 truncate text-xs">{p.name}</span>
            <div className="w-32">
              <AmountInput
                value={inputs[p.id] ?? 0}
                currency={currency}
                onChange={(v) => setExact(p.id, v)}
              />
            </div>
          </div>
        ))}
        <SplitValidation
          valid={valid}
          label={valid ? "Splits sum to total" : `${remaining > 0 ? "+" : ""}${formatAmount(remaining, currency)} remaining`}
        />
      </div>
    );
  }

  // ── Percentage ─────────────────────────────────────────────────────────────
  if (method === "percentage") {
    const sumPct = Object.values(inputs).reduce((s, v) => s + (v || 0), 0);
    const valid = sumPct === 100;

    function setPct(uid: string, v: number) {
      const next = { ...inputs, [uid]: v };
      setInputs(next);
      onChange(participants.map((p) => ({ user_id: p.id, share_units: next[p.id] ?? 0 })));
    }

    return (
      <div className="space-y-2">
        {participants.map((p) => {
          const pct = inputs[p.id] ?? 0;
          const computed = Math.round((pct / 100) * total);
          return (
            <div key={p.id} className="flex items-center gap-2">
              <Avatar name={p.name} size="sm" className="h-5 w-5 text-[0.55rem] flex-shrink-0" />
              <span className="flex-1 truncate text-xs">{p.name}</span>
              <div className="flex items-center gap-1.5 w-36">
                <Input
                  type="number"
                  min={0}
                  max={100}
                  value={pct}
                  onChange={(e) => setPct(p.id, Number(e.target.value))}
                  className="w-14 h-8 px-2 text-xs text-right"
                />
                <span className="text-xs text-[hsl(var(--muted-foreground))]">%</span>
                <span className="text-xs font-mono text-[hsl(var(--muted-foreground))] w-16 text-right truncate">
                  {formatAmount(computed, currency)}
                </span>
              </div>
            </div>
          );
        })}
        <SplitValidation
          valid={valid}
          label={valid ? "Percentages sum to 100%" : `${sumPct}% / 100% — ${valid ? "" : `${100 - sumPct}% remaining`}`}
        />
      </div>
    );
  }

  // ── Shares / Weights ───────────────────────────────────────────────────────
  if (method === "shares") {
    const totalUnits = Object.values(inputs).reduce((s, v) => s + (v || 0), 0);

    function setShares(uid: string, v: number) {
      const next = { ...inputs, [uid]: v };
      setInputs(next);
      onChange(participants.map((p) => ({ user_id: p.id, share_units: next[p.id] ?? 1 })));
    }

    return (
      <div className="space-y-2">
        {participants.map((p) => {
          const units = inputs[p.id] ?? 1;
          const computed = totalUnits > 0 ? Math.round((units / totalUnits) * total) : 0;
          return (
            <div key={p.id} className="flex items-center gap-2">
              <Avatar name={p.name} size="sm" className="h-5 w-5 text-[0.55rem] flex-shrink-0" />
              <span className="flex-1 truncate text-xs">{p.name}</span>
              <div className="flex items-center gap-1.5 w-36">
                <Input
                  type="number"
                  min={1}
                  value={units}
                  onChange={(e) => setShares(p.id, Math.max(1, Number(e.target.value)))}
                  className="w-14 h-8 px-2 text-xs text-right"
                />
                <span className="text-xs text-[hsl(var(--muted-foreground))]">×</span>
                <span className="text-xs font-mono text-[hsl(var(--muted-foreground))] w-16 text-right truncate">
                  {formatAmount(computed, currency)}
                </span>
              </div>
            </div>
          );
        })}
        <div className="flex items-center gap-1 pt-1 text-[hsl(var(--primary))] text-xs">
          <CheckCircle2 className="h-3.5 w-3.5" />
          <span>Proportional by weight</span>
        </div>
      </div>
    );
  }

  return null;
}

function SplitValidation({ valid, label }: { valid: boolean; label: string }) {
  return (
    <div className={cn("flex items-center gap-1 pt-1 text-xs", valid ? "text-[hsl(var(--primary))]" : "text-[hsl(var(--destructive))]")}>
      {valid ? <CheckCircle2 className="h-3.5 w-3.5" /> : <AlertCircle className="h-3.5 w-3.5" />}
      <span>{label}</span>
    </div>
  );
}
