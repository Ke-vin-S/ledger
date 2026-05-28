"use client";

import { useMyBalances } from "@/hooks/useSettlements";
import { DebtBar } from "@/components/settlement/DebtBar";
import { Skeleton } from "@/components/shared/Skeleton";

export default function LoansPage() {
  const { data: balances, isLoading } = useMyBalances();

  return (
    <div className="p-6 space-y-6 max-w-2xl">
      <div>
        <h1 className="text-2xl font-bold">Loans & Balances</h1>
        <p className="text-sm text-[hsl(var(--muted-foreground))] mt-1">
          Your net balances across all teams
        </p>
      </div>

      {isLoading ? (
        <div className="space-y-2">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-14" />
          ))}
        </div>
      ) : !balances?.length ? (
        <p className="text-sm text-[hsl(var(--muted-foreground))]">
          You&apos;re all settled up across all teams.
        </p>
      ) : (
        <div className="space-y-2">
          {balances.map((b) => (
            <DebtBar
              key={b.counterparty_id}
              counterpartyName={b.counterparty_name}
              netAmount={b.net_amount}
            />
          ))}
        </div>
      )}
    </div>
  );
}
