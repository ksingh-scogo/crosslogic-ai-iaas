import { getServerSession } from "next-auth";
import { authOptions } from "../../../lib/auth";
import { fetchNodeSummaries } from "../../../lib/api";
import NodeManager from "../../../components/node-manager";
import { redirect } from "next/navigation";

export const dynamic = "force-dynamic";

export default async function NodesPage() {
  const session = await getServerSession(authOptions);
  
  if (!session) {
    redirect("/api/auth/signin");
  }

  const nodes = await fetchNodeSummaries();

  return <NodeManager initialNodes={nodes} />;
}

