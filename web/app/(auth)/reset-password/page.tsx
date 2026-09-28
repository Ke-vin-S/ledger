"use client";
import { Suspense, useState } from "react";
import { useSearchParams } from "next/navigation";
import { api, ApiRequestError } from "@/lib/api";
import { API_ENDPOINTS } from "@/constants/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

function ResetPasswordForm() {
  const searchParams = useSearchParams();
  const token = searchParams.get("token") ?? "";
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [complete, setComplete] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    if (password.length < 8) {
      setError("Password must be at least 8 characters.");
      return;
    }
    if (password !== confirmation) {
      setError("Passwords do not match.");
      return;
    }
    setSubmitting(true);
    try {
      await api.post(API_ENDPOINTS.auth.passwordReset, {
        token,
        new_password: password,
      });
      setComplete(true);
    } catch (err) {
      setError(
        err instanceof ApiRequestError
          ? err.error.message
          : "Unable to reset password.",
      );
    } finally {
      setSubmitting(false);
    }
  }

  if (complete) {
    return (
      <div className="space-y-4 text-center">
        <h1 className="text-2xl font-bold">Password updated</h1>
        <p className="text-sm text-muted-foreground">
          You can now sign in with your new password.
        </p>
        <Button asChild className="w-full">
          <a href="/login">Go to sign in</a>
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="text-center">
        <h1 className="text-2xl font-bold">Choose a new password</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Use at least 8 characters.
        </p>
      </div>
      {!token && (
        <p className="text-sm text-destructive">
          This reset link is missing its token.
        </p>
      )}
      <form onSubmit={onSubmit} className="space-y-4">
        {error && <p className="text-sm text-destructive">{error}</p>}
        <div className="space-y-1">
          <label htmlFor="new-password" className="text-sm font-medium">
            New password
          </label>
          <Input
            id="new-password"
            type="password"
            autoComplete="new-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </div>
        <div className="space-y-1">
          <label htmlFor="confirm-password" className="text-sm font-medium">
            Confirm password
          </label>
          <Input
            id="confirm-password"
            type="password"
            autoComplete="new-password"
            value={confirmation}
            onChange={(event) => setConfirmation(event.target.value)}
          />
        </div>
        <Button
          type="submit"
          className="w-full"
          disabled={!token || submitting}
        >
          {submitting ? "Updating…" : "Update password"}
        </Button>
      </form>
    </div>
  );
}

export default function ResetPasswordPage() {
  return (
    <Suspense
      fallback={
        <div className="py-8 text-center text-sm text-muted-foreground">
          Loading reset link…
        </div>
      }
    >
      <ResetPasswordForm />
    </Suspense>
  );
}
