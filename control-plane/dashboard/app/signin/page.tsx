"use client";

import { signIn } from "next-auth/react";
import { FormEvent } from "react";

const isDevelopment = 
  process.env.NEXT_PUBLIC_ENVIRONMENT === "development" ||
  process.env.NODE_ENV === "development";

export default function SignInPage() {
  async function handleEmailSignIn(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const email = form.get("email") as string;
    await signIn("email", { email });
  }

  async function handleCredentialsSignIn(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const email = form.get("email") as string;
    const password = form.get("password") as string;
    await signIn("credentials", { 
      email, 
      password,
      redirect: true,
      callbackUrl: "/"
    });
  }

  return (
    <div
      style={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background: "#f1f5f9"
      }}
    >
      <div
        style={{
          width: 360,
          background: "#ffffff",
          padding: 32,
          borderRadius: 16,
          border: "1px solid var(--border-color)",
          boxShadow: "0 10px 30px rgba(15,23,42,0.08)"
        }}
      >
        <h2>Sign in</h2>
        <p style={{ color: "#64748b", marginBottom: 24 }}>
          Use Google SSO or a verified email to access the admin console.
        </p>
        <button
          type="button"
          onClick={() => signIn("google")}
          style={{
            width: "100%",
            padding: "12px 16px",
            borderRadius: 8,
            border: "1px solid var(--border-color)",
            background: "#0f172a",
            color: "white",
            fontWeight: 600,
            marginBottom: 16,
            cursor: "pointer"
          }}
        >
          Continue with Google
        </button>
        <form onSubmit={handleEmailSignIn} style={{ display: "flex", flexDirection: "column", gap: 12, marginBottom: isDevelopment ? 24 : 0 }}>
          <label style={{ fontSize: 14, fontWeight: 600 }}>Work email</label>
          <input
            type="email"
            name="email"
            required
            placeholder="you@company.com"
            style={{
              padding: "10px 12px",
              borderRadius: 8,
              border: "1px solid var(--border-color)"
            }}
          />
          <button
            type="submit"
            style={{
              padding: "10px 16px",
              borderRadius: 8,
              border: "none",
              background: "#2563eb",
              color: "white",
              fontWeight: 600,
              cursor: "pointer"
            }}
          >
            Send magic link
          </button>
        </form>
        
        {isDevelopment && (
          <>
            <div style={{ 
              display: "flex", 
              alignItems: "center", 
              gap: 8, 
              margin: "24px 0",
              color: "#64748b"
            }}>
              <div style={{ flex: 1, height: 1, background: "#e2e8f0" }} />
              <span style={{ fontSize: 12 }}>Development Mode</span>
              <div style={{ flex: 1, height: 1, background: "#e2e8f0" }} />
            </div>
            <form onSubmit={handleCredentialsSignIn} style={{ display: "flex", flexDirection: "column", gap: 12 }}>
              <label style={{ fontSize: 14, fontWeight: 600 }}>Developer Login</label>
              <input
                type="email"
                name="email"
                required
                placeholder="dev@example.com"
                defaultValue="dev@example.com"
                style={{
                  padding: "10px 12px",
                  borderRadius: 8,
                  border: "1px solid var(--border-color)"
                }}
              />
              <input
                type="password"
                name="password"
                required
                placeholder="Any password"
                defaultValue="dev"
                style={{
                  padding: "10px 12px",
                  borderRadius: 8,
                  border: "1px solid var(--border-color)"
                }}
              />
              <button
                type="submit"
                style={{
                  padding: "10px 16px",
                  borderRadius: 8,
                  border: "none",
                  background: "#10b981",
                  color: "white",
                  fontWeight: 600,
                  cursor: "pointer"
                }}
              >
                Developer Login
              </button>
              <p style={{ fontSize: 11, color: "#94a3b8", margin: 0, textAlign: "center" }}>
                Any email/password works in development mode
              </p>
            </form>
          </>
        )}
      </div>
    </div>
  );
}

