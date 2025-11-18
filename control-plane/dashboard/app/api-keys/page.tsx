import { fetchApiKeys } from "../../lib/api";

export const dynamic = "force-dynamic";

export default async function ApiKeysPage() {
  const apiKeys = await fetchApiKeys();

  return (
    <div>
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          marginBottom: 16
        }}
      >
        <div>
          <h2 style={{ margin: 0 }}>API Keys</h2>
          <p style={{ color: "#64748b" }}>
            Rotate keys frequently and scope them per environment.
          </p>
        </div>
        <button
          type="button"
          style={{
            padding: "10px 16px",
            borderRadius: 8,
            border: "none",
            background: "#2563eb",
            color: "white",
            fontWeight: 600,
            cursor: "pointer"
          }}
          onClick={() => {
            alert("Provisioning flow is wired to admin API in server code.");
          }}
        >
          Create key
        </button>
      </div>

      <table className="table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Prefix</th>
            <th>Status</th>
            <th>Created</th>
          </tr>
        </thead>
        <tbody>
          {apiKeys.map((key) => (
            <tr key={key.id}>
              <td>{key.name}</td>
              <td>{key.prefix}</td>
              <td>
                <span
                  className={`pill ${
                    key.status === "active"
                      ? "success"
                      : key.status === "revoked"
                        ? "warn"
                        : "neutral"
                  }`}
                >
                  {key.status}
                </span>
              </td>
              <td>{new Date(key.createdAt).toLocaleString()}</td>
            </tr>
          ))}
          {apiKeys.length === 0 && (
            <tr>
              <td colSpan={4} style={{ textAlign: "center", padding: 24 }}>
                No keys found for the selected tenant.
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}

