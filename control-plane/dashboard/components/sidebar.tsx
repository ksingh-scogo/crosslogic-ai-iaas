"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const navItems = [
  { href: "/", label: "Overview" },
  { href: "/api-keys", label: "API Keys" },
  { href: "/usage", label: "Usage & Billing" }
];

export default function Sidebar() {
  const pathname = usePathname();

  return (
    <aside
      style={{
        width: "var(--sidebar-width)",
        borderRight: "1px solid var(--border-color)",
        background: "#0f172a",
        color: "white",
        padding: "24px 16px",
        display: "flex",
        flexDirection: "column",
        gap: "24px"
      }}
    >
      <div>
        <div style={{ fontSize: 20, fontWeight: 700 }}>CrossLogic</div>
        <div style={{ fontSize: 12, color: "#94a3b8" }}>Inference Cloud</div>
      </div>
      <nav style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        {navItems.map((item) => {
          const active = pathname === item.href;
          return (
            <Link
              key={item.href}
              href={item.href}
              style={{
                padding: "10px 12px",
                borderRadius: 8,
                fontWeight: 600,
                background: active ? "rgba(148, 163, 184, 0.2)" : "transparent",
                color: active ? "#fff" : "#cbd5f5",
                textDecoration: "none"
              }}
            >
              {item.label}
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}

