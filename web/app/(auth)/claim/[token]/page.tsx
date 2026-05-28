"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { UserCheck, AlertCircle, Loader2 } from "lucide-react";

export default function ClaimPage() {
  const { token } = useParams<{ token: string }>();
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleClaim() {
    setLoading(true);
    setError(null);
    try {
      // Backend expects { claim_token } and returns the user profile (no new JWT needed)
      await api.post(`/users/claim`, { claim_token: token });
      router.push("/dashboard");
    } catch (err) {
      if (err instanceof ApiRequestError) {
        setError(err.error.message);
      } else {
        setError("Invalid or expired claim token.");
      }
      setLoading(false);
    }
  }

  if (error) {
    return (
      <div className="text-center space-y-4">
        <div className="flex justify-center">
          <AlertCircle className="h-12 w-12 text-[hsl(var(--destructive))]" />
        </div>
        <h1 className="text-2xl font-bold">Claim Failed</h1>
        <p className="text-[hsl(var(--muted-foreground))] text-sm">{error}</p>
        <a href="/login" className="underline text-sm">
          Go to login
        </a>
      </div>
    );
  }

  return (
    <div className="text-center space-y-6">
      <div className="flex justify-center">
        <div className="h-16 w-16 rounded-full bg-[hsl(var(--primary)/0.1)] flex items-center justify-center">
          <UserCheck className="h-8 w-8 text-[hsl(var(--primary))]" />
        </div>
      </div>
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">Claim your account</h1>
        <p className="text-[hsl(var(--muted-foreground))] text-sm max-w-sm mx-auto">
          This will merge the anonymous placeholder profile into your account, transferring all associated expenses, splits, and balances.
        </p>
      </div>
      <Button onClick={handleClaim} disabled={loading} className="w-full max-w-xs">
        {loading ? (
          <>
            <Loader2 className="h-4 w-4 mr-2 animate-spin" />
            Claiming…
          </>
        ) : (
          "Confirm & claim"
        )}
      </Button>
      <p className="text-xs text-[hsl(var(--muted-foreground))]">
        You must be logged in. This action cannot be undone.
      </p>
    </div>
  );
}
