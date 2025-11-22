"use client";

import { useState } from "react";
import { NodeSummary, LaunchNodeRequest } from "../lib/api";
import { launchNodeAction, terminateNodeAction } from "../app/actions";
import { Server, Power, Activity, Trash2, Plus } from "lucide-react";

export default function NodeManager({ initialNodes }: { initialNodes: NodeSummary[] }) {
  const [nodes, setNodes] = useState<NodeSummary[]>(initialNodes);
  const [isLaunching, setIsLaunching] = useState(false);
  const [launchConfig, setLaunchConfig] = useState<LaunchNodeRequest>({
    provider: "aws",
    region: "us-east-1",
    gpu: "A10G",
    model: "meta-llama/Llama-3-8b-chat-hf",
    use_spot: true
  });

  const handleLaunch = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const result = await launchNodeAction(launchConfig);
      setIsLaunching(false);
      alert(`Node launching! Cluster: ${result.cluster_name}`);
      // Optimistic update
      const newNode: NodeSummary = {
        id: result.node_id,
        status: "launching",
        provider: launchConfig.provider,
        endpoint: "pending...",
        health: 0,
        lastHeartbeat: new Date().toISOString(),
        clusterName: result.cluster_name
      };
      setNodes([newNode, ...nodes]);
    } catch (err) {
      console.error("Failed to launch node", err);
      alert("Failed to launch node");
    }
  };

  const handleTerminate = async (clusterName: string) => {
    if (!confirm(`Terminate node ${clusterName}? This will delete cloud resources.`)) return;
    try {
      await terminateNodeAction(clusterName);
      setNodes(nodes.map(n => n.clusterName === clusterName ? { ...n, status: "terminating" } : n));
    } catch (err) {
      console.error("Failed to terminate node", err);
      alert("Failed to terminate node");
    }
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <div>
          <h2 className="text-2xl font-bold m-0">GPU Nodes</h2>
          <p className="text-slate-500">
            Manage inference infrastructure and SkyPilot clusters.
          </p>
        </div>
        <button
          onClick={() => setIsLaunching(true)}
          className="flex items-center gap-2 bg-blue-600 text-white px-4 py-2 rounded-lg font-medium hover:bg-blue-700 transition-colors"
        >
          <Plus size={16} />
          Launch Node
        </button>
      </div>

      {isLaunching && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white p-6 rounded-xl shadow-xl w-full max-w-lg">
            <h3 className="text-xl font-bold mb-4">Launch GPU Node</h3>
            <form onSubmit={handleLaunch} className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-slate-700 mb-1">Provider</label>
                  <select
                    value={launchConfig.provider}
                    onChange={(e) => setLaunchConfig({ ...launchConfig, provider: e.target.value })}
                    className="w-full border border-slate-300 rounded-lg px-3 py-2"
                  >
                    <option value="aws">AWS</option>
                    <option value="gcp">GCP</option>
                    <option value="azure">Azure</option>
                    <option value="lambda">Lambda Labs</option>
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-slate-700 mb-1">Region</label>
                  <input
                    type="text"
                    value={launchConfig.region}
                    onChange={(e) => setLaunchConfig({ ...launchConfig, region: e.target.value })}
                    className="w-full border border-slate-300 rounded-lg px-3 py-2"
                  />
                </div>
              </div>
              
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-slate-700 mb-1">GPU Type</label>
                  <input
                    type="text"
                    value={launchConfig.gpu}
                    onChange={(e) => setLaunchConfig({ ...launchConfig, gpu: e.target.value })}
                    className="w-full border border-slate-300 rounded-lg px-3 py-2"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-slate-700 mb-1">Spot Instance</label>
                  <div className="flex items-center h-[42px]">
                    <input
                      type="checkbox"
                      checked={launchConfig.use_spot}
                      onChange={(e) => setLaunchConfig({ ...launchConfig, use_spot: e.target.checked })}
                      className="h-5 w-5 text-blue-600 rounded"
                    />
                    <span className="ml-2 text-sm text-slate-600">Use Spot (Save ~70%)</span>
                  </div>
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-700 mb-1">Model</label>
                <input
                  type="text"
                  value={launchConfig.model}
                  onChange={(e) => setLaunchConfig({ ...launchConfig, model: e.target.value })}
                  className="w-full border border-slate-300 rounded-lg px-3 py-2 font-mono text-sm"
                />
              </div>

              <div className="flex gap-3 justify-end mt-6">
                <button
                  type="button"
                  onClick={() => setIsLaunching(false)}
                  className="px-4 py-2 text-slate-600 hover:bg-slate-100 rounded-lg font-medium"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-blue-600 text-white rounded-lg font-medium hover:bg-blue-700"
                >
                  Launch
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      <div className="grid gap-4">
        {nodes.map((node) => (
          <div key={node.id} className="bg-white p-4 rounded-xl border border-slate-200 flex items-center justify-between">
            <div className="flex items-center gap-4">
              <div className={`p-3 rounded-lg ${
                node.status === 'active' ? 'bg-green-100 text-green-600' : 
                node.status === 'launching' ? 'bg-blue-100 text-blue-600' :
                'bg-slate-100 text-slate-600'
              }`}>
                <Server size={24} />
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <h3 className="font-semibold text-slate-900">{node.clusterName || node.id.slice(0, 8)}</h3>
                  <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${
                    node.status === 'active' ? 'bg-green-100 text-green-800' :
                    node.status === 'launching' ? 'bg-blue-100 text-blue-800' :
                    'bg-slate-100 text-slate-800'
                  }`}>
                    {node.status}
                  </span>
                </div>
                <div className="flex items-center gap-4 text-sm text-slate-500 mt-1">
                  <span className="flex items-center gap-1">
                    <Activity size={14} />
                    {node.provider}
                  </span>
                  <span>{node.endpoint}</span>
                </div>
              </div>
            </div>

            <div className="flex items-center gap-6">
              <div className="text-right">
                <div className="text-sm font-medium text-slate-900">Health</div>
                <div className={`text-sm ${node.health > 90 ? 'text-green-600' : 'text-amber-600'}`}>
                  {node.health.toFixed(1)}%
                </div>
              </div>
              
              {node.status !== 'terminated' && node.status !== 'terminating' && (
                <button
                  onClick={() => handleTerminate(node.clusterName || "")}
                  disabled={!node.clusterName}
                  className="p-2 text-slate-400 hover:text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                  title="Terminate Node"
                >
                  <Power size={20} />
                </button>
              )}
            </div>
          </div>
        ))}
        
        {nodes.length === 0 && (
          <div className="text-center py-12 bg-slate-50 rounded-xl border border-dashed border-slate-300 text-slate-500">
            No active nodes found. Launch one to start serving traffic.
          </div>
        )}
      </div>
    </div>
  );
}

