"use client";

import { useState } from "react";
import { ApiKey } from "../../lib/api";
import { createApiKeyAction, revokeApiKeyAction } from "../actions";
import { Trash2, Plus, Copy, Check } from "lucide-react";

export default function ApiKeyManager({ initialKeys }: { initialKeys: ApiKey[] }) {
  const [keys, setKeys] = useState<ApiKey[]>(initialKeys);
  const [isCreating, setIsCreating] = useState(false);
  const [newKeyName, setNewKeyName] = useState("");
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newKeyName.trim()) return;

    try {
      const result = await createApiKeyAction(newKeyName);
      setCreatedKey(result.key);
      setNewKeyName("");
      // Optimistic update or wait for revalidatePath (which happens on server)
      // Since we are in a client component, we might not see the update immediately unless we refresh or update local state.
      // For now, we'll rely on the server revalidation and maybe a router.refresh() if needed, 
      // but let's just add a placeholder to local state to feel responsive.
      const newKey: ApiKey = {
        id: result.id,
        name: newKeyName,
        prefix: "sk-..." + result.key.slice(-4), // Approximation
        created_at: new Date().toISOString(),
        status: "active"
      };
      setKeys([newKey, ...keys]);
    } catch (err) {
      console.error("Failed to create key", err);
      alert("Failed to create key");
    }
  };

  const handleRevoke = async (id: string) => {
    if (!confirm("Are you sure you want to revoke this key? This cannot be undone.")) return;
    try {
      await revokeApiKeyAction(id);
      setKeys(keys.map(k => k.id === id ? { ...k, status: "revoked" } : k));
    } catch (err) {
      console.error("Failed to revoke key", err);
      alert("Failed to revoke key");
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <div>
          <h2 className="text-2xl font-bold m-0">API Keys</h2>
          <p className="text-slate-500">
            Rotate keys frequently and scope them per environment.
          </p>
        </div>
        <button
          onClick={() => setIsCreating(true)}
          className="flex items-center gap-2 bg-blue-600 text-white px-4 py-2 rounded-lg font-medium hover:bg-blue-700 transition-colors"
        >
          <Plus size={16} />
          Create key
        </button>
      </div>

      {isCreating && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white p-6 rounded-xl shadow-xl w-full max-w-md">
            <h3 className="text-xl font-bold mb-4">Create new API key</h3>
            
            {createdKey ? (
              <div className="space-y-4">
                <div className="p-4 bg-green-50 border border-green-200 rounded-lg">
                  <p className="text-green-800 text-sm mb-2 font-medium">
                    Key created successfully! Copy it now, you won't see it again.
                  </p>
                  <div className="flex items-center gap-2 bg-white border p-2 rounded">
                    <code className="flex-1 font-mono text-sm truncate">{createdKey}</code>
                    <button
                      onClick={() => copyToClipboard(createdKey)}
                      className="text-slate-500 hover:text-blue-600"
                    >
                      {copied ? <Check size={16} /> : <Copy size={16} />}
                    </button>
                  </div>
                </div>
                <button
                  onClick={() => {
                    setIsCreating(false);
                    setCreatedKey(null);
                  }}
                  className="w-full bg-slate-900 text-white py-2 rounded-lg font-medium"
                >
                  Done
                </button>
              </div>
            ) : (
              <form onSubmit={handleCreate} className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-slate-700 mb-1">
                    Key Name
                  </label>
                  <input
                    type="text"
                    value={newKeyName}
                    onChange={(e) => setNewKeyName(e.target.value)}
                    placeholder="e.g. Production App"
                    className="w-full border border-slate-300 rounded-lg px-3 py-2 focus:ring-2 focus:ring-blue-500 outline-none"
                    autoFocus
                  />
                </div>
                <div className="flex gap-3 justify-end">
                  <button
                    type="button"
                    onClick={() => setIsCreating(false)}
                    className="px-4 py-2 text-slate-600 hover:bg-slate-100 rounded-lg font-medium"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={!newKeyName.trim()}
                    className="px-4 py-2 bg-blue-600 text-white rounded-lg font-medium hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    Create
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}

      <div className="bg-white rounded-xl border border-slate-200 overflow-hidden">
        <table className="w-full text-left border-collapse">
          <thead className="bg-slate-50 border-b border-slate-200">
            <tr>
              <th className="px-6 py-3 text-sm font-semibold text-slate-600">Name</th>
              <th className="px-6 py-3 text-sm font-semibold text-slate-600">Prefix</th>
              <th className="px-6 py-3 text-sm font-semibold text-slate-600">Status</th>
              <th className="px-6 py-3 text-sm font-semibold text-slate-600">Created</th>
              <th className="px-6 py-3 text-sm font-semibold text-slate-600 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {keys.map((key) => (
              <tr key={key.id} className="hover:bg-slate-50/50">
                <td className="px-6 py-4 font-medium text-slate-900">{key.name}</td>
                <td className="px-6 py-4 font-mono text-sm text-slate-500">{key.prefix}</td>
                <td className="px-6 py-4">
                  <span
                    className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                      key.status === "active"
                        ? "bg-green-100 text-green-800"
                        : "bg-red-100 text-red-800"
                    }`}
                  >
                    {key.status}
                  </span>
                </td>
                <td className="px-6 py-4 text-sm text-slate-500">
                  {new Date(key.created_at).toLocaleDateString()}
                </td>
                <td className="px-6 py-4 text-right">
                  {key.status === "active" && (
                    <button
                      onClick={() => handleRevoke(key.id)}
                      className="text-slate-400 hover:text-red-600 transition-colors"
                      title="Revoke key"
                    >
                      <Trash2 size={18} />
                    </button>
                  )}
                </td>
              </tr>
            ))}
            {keys.length === 0 && (
              <tr>
                <td colSpan={5} className="px-6 py-12 text-center text-slate-500">
                  No API keys found. Create one to get started.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

