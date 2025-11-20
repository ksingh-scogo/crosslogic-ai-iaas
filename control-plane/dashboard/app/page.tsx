import { PlugZap, Shield, ArrowUpRight } from "lucide-react";
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

  const quickstartSnippet = `curl https://api.crosslogic.ai/v1/chat/completions \\
  -H "Authorization: Bearer sk-***" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "mixtral-8x7b", 
    "messages": [{"role": "user", "content": "Ping"}],
    "stream": true
  }'`;

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <div className="eyebrow">Built for engineers</div>
          <h1 style={{ margin: 4, fontSize: 30 }}>CrossLogic developer workspace</h1>
          <p className="muted-text" style={{ maxWidth: 640 }}>
            Opinionated defaults, friendly tooling, and transparent usage so you can ship inference features without fighting the platform.
          </p>
        </div>
        <div className="button-row">
          <a className="btn primary" href="/api-keys">
            Generate key
            <ArrowUpRight size={16} />
          </a>
          <a className="btn secondary" href="/usage">
            Review usage
          </a>
        </div>
      </div>

      <div className="surface emphasis" style={{ marginBottom: 20 }}>
        <div className="quickstart">
          <div>
            <div className="pill neutral" style={{ marginBottom: 10 }}>
              <PlugZap size={14} /> 3-step quickstart
            </div>
            <h3 style={{ margin: "4px 0" }}>Ready-to-run curl example</h3>
            <p className="helper-text">
              Mirrors the OpenAI chat completions contract. Swap models, toggle streaming, and test latency without leaving the console.
            </p>
            <ul style={{ color: "#475569", lineHeight: 1.8, paddingLeft: 18 }}>
              <li>Grab a scoped API key from API Keys</li>
              <li>Send your first chat request</li>
              <li>Watch live usage in this dashboard</li>
            </ul>
          </div>
          <div className="code-block" aria-label="curl quickstart snippet">
            <pre style={{ margin: 0 }}>{quickstartSnippet}</pre>
          </div>
        </div>
      </div>

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

      <div className="surface" style={{ marginTop: 10, marginBottom: 16 }}>
        <div style={{ display: "flex", gap: 14, alignItems: "center", marginBottom: 6 }}>
          <Shield size={18} color="#0f8bff" />
          <strong>Operational posture</strong>
        </div>
        <p className="helper-text" style={{ margin: 0 }}>
          Transparent capacity, explicit billing, and safe defaults for production. Status is mirrored from our internal runbooks.
        </p>
        <div style={{ display: "flex", gap: 12, marginTop: 10, flexWrap: "wrap" }}>
          <span className="pill success">99.99% uptime last 7d</span>
          <span className="pill neutral">Latency: P95 840ms</span>
          <span className="pill neutral">Autoscaling: enabled</span>
        </div>
      </div>

      <div style={{ marginTop: 8 }}>
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

