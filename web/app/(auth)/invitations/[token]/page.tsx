"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAcceptInvitation } from "@/hooks/useTeam";
import { ApiRequestError } from "@/lib/api";
import { ROUTES } from "@/constants/routes";
import { Button } from "@/components/ui/button";
import { Users, CheckCircle2, AlertCircle, Loader2 } from "lucide-react";

export default function AcceptInvitationPage() {
  const { token } = useParams<{ token: string }>();
  const router = useRouter();
  const { mutateAsync: accept } = useAcceptInvitation();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  async function handleAccept() {
    setLoading(true);
    setError(null);
    try {
      const res = await accept(token);
      setSuccess(true);
      setTimeout(() => router.push(ROUTES.team(res.team_id) as never), 1500);
    } catch (err) {
      // If the user isn't signed in, the API layer redirects to /login?next=…
      // and we never reach here. Otherwise the invitation is invalid/expired.
      if (err instanceof ApiRequestError) {
        setError(err.error.message);
      } else {
        setError("This invitation is invalid or has expired.");
      }
      setLoading(false);
    }
  }

  if (success) {
    return (
      <div className="text-center space-y-4">
        <div className="flex justify-center">
          <CheckCircle2 className="h-12 w-12 text-[hsl(var(--primary))]" />
        </div>
        <h1 className="text-2xl font-bold">You&apos;re in!</h1>
        <p className="text-[hsl(var(--muted-foreground))] text-sm">Redirecting to the team…</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center space-y-4">
        <div className="flex justify-center">
          <AlertCircle className="h-12 w-12 text-[hsl(var(--destructive))]" />
        </div>
        <h1 className="text-2xl font-bold">Invitation Invalid</h1>
        <p className="text-[hsl(var(--muted-foreground))] text-sm">{error}</p>
        <a href={ROUTES.dashboard} className="underline text-sm">Go to dashboard</a>
      </div>
    );
  }

  return (
    <div className="text-center space-y-6">
      <div className="flex justify-center">
        <div className="h-16 w-16 rounded-full bg-[hsl(var(--primary)/0.1)] flex items-center justify-center">
          <Users className="h-8 w-8 text-[hsl(var(--primary))]" />
        </div>
      </div>
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">You&apos;ve been invited</h1>
        <p className="text-[hsl(var(--muted-foreground))] text-sm max-w-sm mx-auto">
          Accept this invitation to join the team and start splitting expenses together.
          You&apos;ll be asked to sign in or create an account if you haven&apos;t already.
        </p>
      </div>
      <Button onClick={handleAccept} disabled={loading} className="w-full max-w-xs">
        {loading ? (
          <>
            <Loader2 className="h-4 w-4 mr-2 animate-spin" />
            Joining…
          </>
        ) : (
          "Accept invitation"
        )}
      </Button>
    </div>
  );
}
