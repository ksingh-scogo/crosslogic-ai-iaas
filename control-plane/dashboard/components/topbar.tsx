"use client";

import { signOut } from "next-auth/react";

export default function Topbar() {
  return (
    <header
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        padding: "16px 24px",
        borderBottom: "1px solid var(--border-color)",
        background: "#ffffff"
      }}
    >
      <div>
        <div style={{ fontSize: 14, color: "#64748b" }}>Admin Console</div>
        <div style={{ fontSize: 20, fontWeight: 600 }}>CrossLogic Inference</div>
      </div>
      <button
        type="button"
        onClick={() => signOut()}
        style={{
          border: "1px solid var(--border-color)",
          borderRadius: 8,
          padding: "8px 14px",
          background: "#f8fafc",
          cursor: "pointer",
          fontWeight: 600
        }}
      >
        Sign out
      </button>
    </header>
  );
}

