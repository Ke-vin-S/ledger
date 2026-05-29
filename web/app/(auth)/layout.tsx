import { GoogleOAuthProvider } from "@react-oauth/google";
import { GOOGLE_CLIENT_ID } from "@/constants/config";

export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <GoogleOAuthProvider clientId={GOOGLE_CLIENT_ID}>
      <div className="min-h-screen flex items-center justify-center bg-[hsl(var(--background))] px-4">
        <div className="w-full max-w-md">{children}</div>
      </div>
    </GoogleOAuthProvider>
  );
}
