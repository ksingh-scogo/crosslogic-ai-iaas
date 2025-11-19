import { getServerSession } from "next-auth";
import { authOptions } from "../../lib/auth";
import { fetchApiKeys } from "../../lib/api";
import ApiKeyManager from "../../components/api-key-manager";
import { redirect } from "next/navigation";

export const dynamic = "force-dynamic";

export default async function ApiKeysPage() {
  const session = await getServerSession(authOptions);
  
  if (!session) {
    redirect("/api/auth/signin");
  }

  const tenantId = (session.user as any).tenantId;
  const apiKeys = await fetchApiKeys(tenantId);

  return <ApiKeyManager initialKeys={apiKeys} />;
}

