import { fetchUsageHistory } from "../../lib/api";

const DEFAULT_TENANT =
  process.env.CROSSLOGIC_DASHBOARD_TENANT_ID ??
  "00000000-0000-0000-0000-000000000000";

export default async function UsagePage() {
  const usage = await fetchUsageHistory(DEFAULT_TENANT);

  return (
    <div>
      <h2>Usage & Billing</h2>
      <p style={{ color: "#64748b", marginBottom: 16 }}>
        Streaming requests are now billed using live token counters from vLLM.
      </p>
      <table className="table">
        <thead>
          <tr>
            <th>Timestamp</th>
            <th>Prompt tokens</th>
            <th>Completion tokens</th>
            <th>Total cost</th>
          </tr>
        </thead>
        <tbody>
          {usage.map((point) => (
            <tr key={point.timestamp}>
              <td>{new Date(point.timestamp).toLocaleString()}</td>
              <td>{point.promptTokens.toLocaleString()}</td>
              <td>{point.completionTokens.toLocaleString()}</td>
              <td>{point.totalCost}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

