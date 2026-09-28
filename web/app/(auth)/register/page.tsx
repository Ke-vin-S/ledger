"use client";

import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { api, ApiRequestError } from "@/lib/api";
import { setAccessToken } from "@/lib/auth";
import { ROUTES } from "@/constants/routes";
import { readNextParam } from "@/lib/next-path";
import { GoogleSignInButton } from "@/components/auth/GoogleSignInButton";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

const schema = z.object({
  display_name: z.string().min(1, "Name is required").max(80, "Name too long"),
  email: z.string().email("Invalid email"),
  password: z.string().min(8, "Password must be at least 8 characters"),
});
type FormValues = z.infer<typeof schema>;

export default function RegisterPage() {
  const router = useRouter();
  const [serverError, setServerError] = useState<string | null>(null);
  const [nextPath, setNextPath] = useState<string | null>(null);
  useEffect(() => setNextPath(readNextParam()), []);
  const loginHref = nextPath
    ? `${ROUTES.login}?next=${encodeURIComponent(nextPath)}`
    : ROUTES.login;
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver: zodResolver(schema) });

  async function onSubmit(data: FormValues) {
    setServerError(null);
    try {
      const response = await api.post<{ access_token: string }>(
        "/auth/register",
        data,
      );
      setAccessToken(response.access_token);
      router.push((nextPath ?? ROUTES.dashboard) as never);
    } catch (err) {
      setServerError(
        err instanceof ApiRequestError
          ? err.error.message
          : "Something went wrong. Please try again.",
      );
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <p className="text-sm font-medium text-primary">Start together</p>
        <h1 className="mt-1 font-display text-3xl font-semibold tracking-tight">
          Create your account
        </h1>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">
          Bring your team&apos;s expenses and balances into focus.
        </p>
      </div>
      <GoogleSignInButton onError={setServerError} />
      <div className="flex items-center gap-3" aria-hidden="true">
        <div className="h-px flex-1 bg-border" />
        <span className="text-xs text-muted-foreground">or use email</span>
        <div className="h-px flex-1 bg-border" />
      </div>
      <form noValidate onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        {serverError ? (
          <p
            role="alert"
            className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive"
          >
            {serverError}
          </p>
        ) : null}
        <div className="space-y-1.5">
          <Label htmlFor="register-name">Display name</Label>
          <Input
            id="register-name"
            autoComplete="name"
            placeholder="Your name"
            aria-invalid={errors.display_name ? true : undefined}
            {...register("display_name")}
          />
          {errors.display_name ? (
            <p className="text-xs text-destructive">
              {errors.display_name.message}
            </p>
          ) : null}
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="register-email">Email</Label>
          <Input
            id="register-email"
            type="email"
            autoComplete="email"
            placeholder="you@example.com"
            aria-invalid={errors.email ? true : undefined}
            {...register("email")}
          />
          {errors.email ? (
            <p className="text-xs text-destructive">{errors.email.message}</p>
          ) : null}
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="register-password">Password</Label>
          <Input
            id="register-password"
            type="password"
            autoComplete="new-password"
            placeholder="At least 8 characters"
            aria-invalid={errors.password ? true : undefined}
            {...register("password")}
          />
          {errors.password ? (
            <p className="text-xs text-destructive">
              {errors.password.message}
            </p>
          ) : null}
        </div>
        <Button type="submit" disabled={isSubmitting} className="w-full">
          {isSubmitting ? "Creating account…" : "Create account"}
        </Button>
      </form>
      <p className="text-center text-sm text-muted-foreground">
        Already have an account?{" "}
        <Link
          href={loginHref as never}
          className="font-medium text-foreground underline underline-offset-4 hover:text-primary"
        >
          Sign in
        </Link>
      </p>
    </div>
  );
}
