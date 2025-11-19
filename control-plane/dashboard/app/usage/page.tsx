import { getServerSession } from "next-auth";
import { authOptions } from "../../lib/auth";
import { fetchUsageHistory } from "../../lib/api";
import UsageChart from "../../components/usage-chart";
import { redirect } from "next/navigation";

export default async function UsagePage() {
  const session = await getServerSession(authOptions);
  
  if (!session) {
    redirect("/api/auth/signin");
  }

  const tenantId = (session.user as any).tenantId;
  const usage = await fetchUsageHistory(tenantId);

  return (
    <div>
      <div className="mb-8">
        <h2 className="text-2xl font-bold m-0">Usage & Billing</h2>
        <p className="text-slate-500 mt-1">
          Streaming requests are now billed using live token counters from vLLM.
        </p>
      </div>

      <UsageChart data={usage} />

      <div className="bg-white rounded-xl border border-slate-200 overflow-hidden">
        <table className="w-full text-left border-collapse">
          <thead className="bg-slate-50 border-b border-slate-200">
            <tr>
              <th className="px-6 py-3 text-sm font-semibold text-slate-600">Timestamp</th>
              <th className="px-6 py-3 text-sm font-semibold text-slate-600">Prompt tokens</th>
              <th className="px-6 py-3 text-sm font-semibold text-slate-600">Completion tokens</th>
              <th className="px-6 py-3 text-sm font-semibold text-slate-600">Total cost</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {usage.map((point) => (
              <tr key={point.timestamp} className="hover:bg-slate-50/50">
                <td className="px-6 py-4 text-sm text-slate-900">
                  {new Date(point.timestamp).toLocaleString()}
                </td>
                <td className="px-6 py-4 text-sm text-slate-600">{point.promptTokens.toLocaleString()}</td>
                <td className="px-6 py-4 text-sm text-slate-600">{point.completionTokens.toLocaleString()}</td>
                <td className="px-6 py-4 text-sm font-medium text-slate-900">{point.totalCost}</td>
              </tr>
            ))}
            {usage.length === 0 && (
              <tr>
                <td colSpan={4} className="px-6 py-12 text-center text-slate-500">
                  No usage data available for this period.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

