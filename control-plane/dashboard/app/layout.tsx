import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import Providers from "../components/providers";
import Sidebar from "../components/sidebar";
import Topbar from "../components/topbar";

const inter = Inter({ subsets: ["latin"] });

export const metadata: Metadata = {
  title: "CrossLogic Dashboard",
  description: "Admin console for CrossLogic Inference Cloud"
};

export default function RootLayout({
  children
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className={inter.className}>
        <Providers>
          <div className="app-shell">
            <Sidebar />
            <main className="app-main">
              <Topbar />
              <section className="app-content">{children}</section>
            </main>
          </div>
        </Providers>
      </body>
    </html>
  );
}

