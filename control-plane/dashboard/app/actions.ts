"use server";

import { getServerSession } from "next-auth";
import { authOptions } from "../lib/auth";
import { createApiKey, revokeApiKey, launchNode, terminateNode, LaunchNodeRequest } from "../lib/api";
import { revalidatePath } from "next/cache";

export async function createApiKeyAction(name: string) {
  const session = await getServerSession(authOptions);
  if (!session || !(session.user as any).tenantId) {
    throw new Error("Unauthorized");
  }

  const tenantId = (session.user as any).tenantId;
  const result = await createApiKey(tenantId, name);
  revalidatePath("/api-keys");
  return result;
}

export async function revokeApiKeyAction(keyId: string) {
  const session = await getServerSession(authOptions);
  if (!session) {
    throw new Error("Unauthorized");
  }

  await revokeApiKey(keyId);
  revalidatePath("/api-keys");
}

export async function launchNodeAction(req: LaunchNodeRequest) {
  const session = await getServerSession(authOptions);
  // TODO: Add admin check here. For MVP dev, we assume authenticated users can do this.
  if (!session) {
    throw new Error("Unauthorized");
  }

  const result = await launchNode(req);
  revalidatePath("/admin/nodes");
  return result;
}

export async function terminateNodeAction(clusterName: string) {
  const session = await getServerSession(authOptions);
  if (!session) {
    throw new Error("Unauthorized");
  }

  await terminateNode(clusterName);
  revalidatePath("/admin/nodes");
}
