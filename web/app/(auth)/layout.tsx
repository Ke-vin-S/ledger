import Link from "next/link";
import { CircleDollarSign } from "lucide-react";
import { GoogleOAuthProvider } from "@react-oauth/google";
import { GOOGLE_CLIENT_ID } from "@/constants/config";
import { Card, CardContent } from "@/components/ui/card";

export default function AuthLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <GoogleOAuthProvider clientId={GOOGLE_CLIENT_ID}>
      <main className="relative flex min-h-screen items-center justify-center overflow-hidden bg-background px-4 py-8 sm:px-6">
        <div
          className="pointer-events-none absolute -left-24 top-[-10rem] size-80 rounded-full bg-primary/15 blur-3xl"
          aria-hidden="true"
        />
        <div
          className="pointer-events-none absolute -right-32 bottom-[-12rem] size-96 rounded-full bg-secondary/20 blur-3xl"
          aria-hidden="true"
        />
        <div className="relative z-10 w-full max-w-md">
          <Link
            href="/"
            className="mb-8 flex items-center justify-center gap-2 text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            aria-label="SplitLedger home"
          >
            <span className="flex size-10 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-sm">
              <CircleDollarSign className="h-5 w-5" aria-hidden="true" />
            </span>
            <span className="font-display text-2xl font-semibold tracking-tight">
              SplitLedger
            </span>
          </Link>
          <div className="mb-6 text-center">
            <p className="text-sm font-medium text-primary">
              Shared money, made clear.
            </p>
            <p className="mt-2 text-sm leading-6 text-muted-foreground">
              Track expenses, settle balances, and keep every team in sync.
            </p>
          </div>
          <Card className="border-border/80 bg-card/95 shadow-xl shadow-primary/5 backdrop-blur">
            <CardContent className="p-5 sm:p-8">{children}</CardContent>
          </Card>
          <p className="mt-6 text-center text-xs text-muted-foreground">
            Your financial history stays yours.
          </p>
        </div>
      </main>
    </GoogleOAuthProvider>
  );
}
