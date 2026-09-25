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
  email: z.string().email("Invalid email"),
  password: z.string().min(1, "Password is required"),
});
type FormValues = z.infer<typeof schema>;

export default function LoginPage() {
  const router = useRouter();
  const [serverError, setServerError] = useState<string | null>(null);
  const [nextPath, setNextPath] = useState<string | null>(null);
  useEffect(() => setNextPath(readNextParam()), []);
  const registerHref = nextPath
    ? `${ROUTES.register}?next=${encodeURIComponent(nextPath)}`
    : ROUTES.register;
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver: zodResolver(schema) });

  async function onSubmit(data: FormValues) {
    setServerError(null);
    try {
      const response = await api.post<{ access_token: string }>(
        "/auth/login",
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
        <p className="text-sm font-medium text-primary">Welcome back</p>
        <h1 className="mt-1 font-display text-3xl font-semibold tracking-tight">
          Sign in to continue
        </h1>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">
          Pick up where your shared balances left off.
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
          <Label htmlFor="login-email">Email</Label>
          <Input
            id="login-email"
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
          <Label htmlFor="login-password">Password</Label>
          <Input
            id="login-password"
            type="password"
            autoComplete="current-password"
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
          {isSubmitting ? "Signing in…" : "Sign in"}
        </Button>
      </form>
      <div className="space-y-2 text-center text-sm text-muted-foreground">
        <p>
          Don&apos;t have an account?{" "}
          <Link
            href={registerHref as never}
            className="font-medium text-foreground underline underline-offset-4 hover:text-primary"
          >
            Register
          </Link>
        </p>
        <Link
          href={ROUTES.resetPassword}
          className="inline-block underline underline-offset-4 hover:text-foreground"
        >
          Forgot your password?
        </Link>
      </div>
    </div>
  );
}
