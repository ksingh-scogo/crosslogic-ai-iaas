import { fetchNodeSummaries, fetchUsageHistory } from "../lib/api";
import { StatCard } from "../components/cards";

const DEFAULT_TENANT =
  process.env.CROSSLOGIC_DASHBOARD_TENANT_ID ??
  "00000000-0000-0000-0000-000000000000";

export default async function Home() {
  const [usage, nodes] = await Promise.all([
    fetchUsageHistory(DEFAULT_TENANT),
    fetchNodeSummaries()
  ]);

  const totalPrompt = usage.reduce((acc, point) => acc + point.promptTokens, 0);
  const totalCompletion = usage.reduce(
    (acc, point) => acc + point.completionTokens,
    0
  );

  return (
    <div>
      <div className="card-grid">
        <StatCard
          label="Prompt tokens (24h)"
          value={totalPrompt.toLocaleString()}
          subtext="Aggregated across all environments"
        />
        <StatCard
          label="Completion tokens (24h)"
          value={totalCompletion.toLocaleString()}
          subtext="Streaming + non-streaming"
        />
        <StatCard
          label="Active nodes"
          value={nodes.filter((node) => node.status === "active").length.toString()}
          subtext={`${nodes.length} total provisioned`}
        />
        <StatCard
          label="Estimated spend"
          value={`$${(
            usage.reduce(
              (acc, point) => acc + parseFloat(point.totalCost.replace("$", "")),
              0
            ) || 0
          ).toFixed(2)}`}
          subtext="Based on live meter rates"
        />
      </div>

      <div style={{ marginTop: 24 }}>
        <h2 style={{ marginBottom: 12 }}>Recent Usage</h2>
        <table className="table">
          <thead>
            <tr>
              <th>Timestamp</th>
              <th>Prompt tokens</th>
              <th>Completion tokens</th>
              <th>Cost</th>
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
    </div>
  );
}

