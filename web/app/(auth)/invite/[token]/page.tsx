"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { api, ApiRequestError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Users, CheckCircle2, AlertCircle, Loader2 } from "lucide-react";

type TeamPreview = {
  team_id: string;
  team_name: string;
  description?: string;
  member_count: number;
};

export default function InvitePage() {
  const { token } = useParams<{ token: string }>();
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [teamId, setTeamId] = useState<string | null>(null);

  async function handleAccept() {
    setLoading(true);
    setError(null);
    try {
      const res = await api.post<{ team_id: string }>(`/invite/${token}`);
      setTeamId(res.team_id);
      setSuccess(true);
      setTimeout(() => router.push(`/teams/${res.team_id}`), 1500);
    } catch (err) {
      if (err instanceof ApiRequestError) {
        setError(err.error.message);
      } else {
        setError("This invite link is invalid or has expired.");
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
        <h1 className="text-2xl font-bold">Invite Invalid</h1>
        <p className="text-[hsl(var(--muted-foreground))] text-sm">{error}</p>
        <a href="/dashboard" className="underline text-sm">Go to dashboard</a>
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
      <p className="text-xs text-[hsl(var(--muted-foreground))]">
        You must be logged in to accept this invitation.{" "}
        <a href="/login" className="underline">Log in</a>
      </p>
    </div>
  );
}
