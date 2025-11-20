"use client";

import { signOut } from "next-auth/react";
import Link from "next/link";
import { Sparkles, ArrowUpRight, BookOpen } from "lucide-react";

export default function Topbar() {
  return (
    <header
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        padding: "18px 28px",
        borderBottom: "1px solid var(--border-color)",
        background: "#ffffffd9",
        backdropFilter: "blur(12px)",
        position: "sticky",
        top: 0,
        zIndex: 10
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 14 }}>
        <span className="pill neutral" style={{ fontWeight: 700 }}>
          <Sparkles size={14} /> Developer-first
        </span>
        <div>
          <div style={{ fontSize: 14, color: "#64748b" }}>Admin Console</div>
          <div style={{ fontSize: 20, fontWeight: 700 }}>CrossLogic Inference</div>
        </div>
      </div>
      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        <Link
          href="/usage"
          className="btn secondary"
          style={{ background: "#f8fafc" }}
        >
          <BookOpen size={16} /> Docs & Limits
        </Link>
        <button
          type="button"
          onClick={() => signOut()}
          className="btn primary"
        >
          Sign out
          <ArrowUpRight size={16} />
        </button>
      </div>
    </header>
  );
}

