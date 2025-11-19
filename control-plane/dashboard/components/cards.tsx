type StatCardProps = {
  label: string;
  value: string;
  subtext?: string;
};

export function StatCard({ label, value, subtext }: StatCardProps) {
  return (
    <div className="data-card">
      <span style={{ fontSize: 14, color: "#64748b" }}>{label}</span>
      <strong style={{ fontSize: 28 }}>{value}</strong>
      {subtext && <span style={{ fontSize: 12, color: "#94a3b8" }}>{subtext}</span>}
    </div>
  );
}

type UsageRowProps = {
  timestamp: string;
  promptTokens: number;
  completionTokens: number;
  totalCost: string;
};

export function UsageRow({
  timestamp,
  promptTokens,
  completionTokens,
  totalCost
}: UsageRowProps) {
  return (
    <tr>
      <td>{timestamp}</td>
      <td>{promptTokens.toLocaleString()}</td>
      <td>{completionTokens.toLocaleString()}</td>
      <td>{totalCost}</td>
    </tr>
  );
}

