import type { NextAuthOptions } from "next-auth";
import EmailProvider from "next-auth/providers/email";
import GoogleProvider from "next-auth/providers/google";
import CredentialsProvider from "next-auth/providers/credentials";
import { resolveTenant } from "./api";

export const authOptions: NextAuthOptions = {
  providers: [
    GoogleProvider({
      clientId: process.env.GOOGLE_CLIENT_ID ?? "",
      clientSecret: process.env.GOOGLE_CLIENT_SECRET ?? ""
    }),
    EmailProvider({
      server: process.env.EMAIL_SERVER,
      from: process.env.EMAIL_FROM
    }),
    CredentialsProvider({
      name: "Development",
      credentials: {
        email: { label: "Email", type: "text", placeholder: "dev@example.com" },
        password: { label: "Password", type: "password" }
      },
      async authorize(credentials) {
        // This is for development only
        if (process.env.NODE_ENV === "development") {
          return {
            id: "dev-user-1",
            name: "Dev User",
            email: credentials?.email,
            // Assign a fixed tenant ID for development
            tenantId: "00000000-0000-0000-0000-000000000000" 
          };
        }
        return null;
      }
    })
  ],
  pages: {
    signIn: "/signin"
  },
  session: {
    strategy: "jwt"
  },
  callbacks: {
    async jwt({ token, user }) {
      if (user) {
        token.id = user.id;
        // If user has a tenantId (from Credentials provider), use it
        if ((user as any).tenantId) {
          token.tenantId = (user as any).tenantId;
        } else if (user.email) {
          // Otherwise, resolve tenant from backend
          try {
            const tenant = await resolveTenant(user.email, user.name || "User");
            token.tenantId = tenant.id;
          } catch (err) {
            console.error("Failed to resolve tenant", err);
            // Fallback for now, or handle error appropriately
          }
        }
      }
      return token;
    },
    async session({ session, token }) {
      if (session.user) {
        (session.user as any).id = token.id;
        (session.user as any).tenantId = token.tenantId;
      }
      return session;
    }
  }
};
