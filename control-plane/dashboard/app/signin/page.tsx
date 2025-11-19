"use client";

import { signIn } from "next-auth/react";
import { FormEvent } from "react";

export default function SignInPage() {
  async function handleEmailSignIn(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const email = form.get("email") as string;
    await signIn("email", { email });
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
        <form onSubmit={handleEmailSignIn} style={{ display: "flex", flexDirection: "column", gap: 12 }}>
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
      </div>
    </div>
  );
}

