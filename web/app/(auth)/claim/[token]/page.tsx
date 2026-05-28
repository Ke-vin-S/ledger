"use client";

import { useEffect, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { api, ApiRequestError } from "@/lib/api";
import { setAccessToken } from "@/lib/auth";

export default function ClaimPage() {
  const { token } = useParams<{ token: string }>();
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);
  const claimed = useRef(false);

  useEffect(() => {
    if (claimed.current) return;
    claimed.current = true;

    api
      .post<{ access_token: string }>(`/users/claim/${token}`)
      .then((res) => {
        setAccessToken(res.access_token);
        router.push("/dashboard");
      })
      .catch((err) => {
        if (err instanceof ApiRequestError) {
          setError(err.error.message);
        } else {
          setError("Invalid or expired claim token.");
        }
      });
  }, [token, router]);

  if (error) {
    return (
      <div className="text-center space-y-4">
        <h1 className="text-2xl font-bold">Claim Failed</h1>
        <p className="text-[hsl(var(--destructive))]">{error}</p>
        <a href="/login" className="underline text-sm">
          Go to login
        </a>
      </div>
    );
  }

  return (
    <div className="text-center space-y-4">
      <h1 className="text-2xl font-bold">Claiming your account…</h1>
      <p className="text-sm text-[hsl(var(--muted-foreground))]">Please wait a moment.</p>
    </div>
  );
}
