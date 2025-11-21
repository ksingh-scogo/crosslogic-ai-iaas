"use client";

import { signIn } from "next-auth/react";
import { useSession } from "next-auth/react";
import { Code } from "lucide-react";

const isDevelopment = 
  process.env.NEXT_PUBLIC_ENVIRONMENT === "development" ||
  process.env.NODE_ENV === "development";

export default function DeveloperLogin() {
  const { data: session, status } = useSession();

  // Only show in development mode and when not authenticated
  if (!isDevelopment || status === "authenticated") {
    return null;
  }

  async function handleDeveloperLogin() {
    await signIn("credentials", {
      email: "dev@example.com",
      password: "dev",
      redirect: true,
      callbackUrl: "/"
    });
  }

  return (
    <div
      style={{
        position: "fixed",
        top: 20,
        right: 20,
        zIndex: 1000
      }}
    >
      <button
        type="button"
        onClick={handleDeveloperLogin}
        style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          padding: "10px 16px",
          borderRadius: 8,
          border: "none",
          background: "#10b981",
          color: "white",
          fontWeight: 600,
          cursor: "pointer",
          boxShadow: "0 4px 12px rgba(16, 185, 129, 0.3)",
          fontSize: 14
        }}
      >
        <Code size={16} />
        Developer Login
      </button>
    </div>
  );
}

