"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { LayoutDashboard, KeyRound, Gauge, ArrowUpRight } from "lucide-react";

const navItems = [
  { href: "/", label: "Overview", icon: LayoutDashboard },
  { href: "/api-keys", label: "API Keys", icon: KeyRound },
  { href: "/usage", label: "Usage & Billing", icon: Gauge }
];

export default function Sidebar() {
  const pathname = usePathname();

  return (
    <aside
      style={{
        width: "var(--sidebar-width)",
        borderRight: "1px solid var(--border-color)",
        background: "#0b1626",
        color: "white",
        padding: "26px 18px",
        display: "flex",
        flexDirection: "column",
        gap: "24px"
      }}
    >
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div>
          <div style={{ fontSize: 20, fontWeight: 800 }}>CrossLogic</div>
          <div style={{ fontSize: 12, color: "#94a3b8", letterSpacing: 0.2 }}>
            Developer Cloud
          </div>
        </div>
        <span
          className="pill"
          style={{ background: "rgba(34,197,94,0.12)", color: "#4ade80", borderColor: "rgba(34,197,94,0.32)" }}
        >
          Live
        </span>
      </div>
      <nav style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {navItems.map((item) => {
          const active = pathname === item.href;
          const Icon = item.icon;
          return (
            <Link
              key={item.href}
              href={item.href}
              style={{
                padding: "12px 14px",
                borderRadius: 10,
                fontWeight: 600,
                background: active ? "rgba(255,255,255,0.08)" : "transparent",
                color: "#e2e8f0",
                textDecoration: "none",
                display: "flex",
                gap: 10,
                alignItems: "center",
                border: active ? "1px solid rgba(255,255,255,0.18)" : "1px solid transparent"
              }}
            >
              <Icon size={16} />
              <span>{item.label}</span>
            </Link>
          );
        })}
      </nav>

      <div
        style={{
          marginTop: "auto",
          padding: "14px",
          borderRadius: 12,
          border: "1px solid rgba(255,255,255,0.08)",
          background: "linear-gradient(180deg, rgba(255,255,255,0.06), rgba(148,163,184,0.08))",
          color: "#cbd5e1",
          fontSize: 13,
          display: "flex",
          flexDirection: "column",
          gap: 8
        }}
      >
        <div style={{ fontWeight: 700, color: "#e2e8f0" }}>Need help?</div>
        <div style={{ lineHeight: 1.5 }}>
          Reach us via the platform support channel — typical response under 30 min.
        </div>
        <Link
          href="/api-keys"
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: 6,
            color: "#bfdbfe",
            fontWeight: 700,
            textDecoration: "none"
          }}
        >
          Open developer setup
          <ArrowUpRight size={16} />
        </Link>
      </div>
    </aside>
  );
}

