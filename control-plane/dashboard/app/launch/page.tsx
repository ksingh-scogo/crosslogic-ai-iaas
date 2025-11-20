'use client';

import { useState, useEffect } from 'react';

interface Model {
  id: string;
  name: string;
  family: string;
  size: string;
  type: string;
  vram_required_gb: number;
}

interface LaunchConfig {
  model_name: string;
  provider: string;
  region: string;
  instance_type: string;
  use_spot: boolean;
}

export default function LaunchPage() {
  const [models, setModels] = useState<Model[]>([]);
  const [selectedModel, setSelectedModel] = useState<string>('');
  const [config, setConfig] = useState<LaunchConfig>({
    model_name: '',
    provider: 'azure',
    region: 'eastus',
    instance_type: 'Standard_NV36ads_A10_v5',
    use_spot: true,
  });
  const [launching, setLaunching] = useState(false);
  const [status, setStatus] = useState<any>(null);
  const [jobId, setJobId] = useState<string>('');

  // Fetch available models
  useEffect(() => {
    fetchModels();
  }, []);

  const fetchModels = async () => {
    try {
      const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL || '/api'}/admin/models/r2`, {
        headers: {
          'X-Admin-Token': process.env.NEXT_PUBLIC_ADMIN_TOKEN || '',
        },
      });
      const data = await response.json();
      setModels(data.models || []);
    } catch (error) {
      console.error('Failed to fetch models:', error);
    }
  };

  const handleLaunch = async () => {
    setLaunching(true);
    setStatus(null);

    try {
      const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL || '/api'}/admin/instances/launch`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Admin-Token': process.env.NEXT_PUBLIC_ADMIN_TOKEN || '',
        },
        body: JSON.stringify({
          ...config,
          model_name: selectedModel,
        }),
      });

      const data = await response.json();
      setJobId(data.job_id);
      
      // Start polling for status
      pollStatus(data.job_id);
    } catch (error) {
      console.error('Launch failed:', error);
      setLaunching(false);
    }
  };

  const pollStatus = async (jid: string) => {
    const interval = setInterval(async () => {
      try {
        const response = await fetch(
          `${process.env.NEXT_PUBLIC_API_URL || '/api'}/admin/instances/status?job_id=${jid}`,
          {
            headers: {
              'X-Admin-Token': process.env.NEXT_PUBLIC_ADMIN_TOKEN || '',
            },
          }
        );
        const data = await response.json();
        setStatus(data);

        if (data.status === 'completed' || data.status === 'failed') {
          clearInterval(interval);
          setLaunching(false);
        }
      } catch (error) {
        console.error('Status check failed:', error);
        clearInterval(interval);
        setLaunching(false);
      }
    }, 3000); // Poll every 3 seconds
  };

  const providerOptions = {
    azure: {
      regions: ['eastus', 'westus2', 'centralus'],
      instances: [
        'Standard_NV36ads_A10_v5',
        'Standard_NC6s_v3',
        'Standard_NC24s_v3',
      ],
    },
    aws: {
      regions: ['us-east-1', 'us-west-2', 'eu-west-1'],
      instances: ['g4dn.xlarge', 'g4dn.2xlarge', 'g5.xlarge', 'g5.2xlarge'],
    },
    gcp: {
      regions: ['us-central1', 'us-west1', 'europe-west1'],
      instances: ['n1-standard-4', 'n1-standard-8', 'a2-highgpu-1g'],
    },
  };

  return (
    <div className="max-w-4xl mx-auto p-6">
      <h1 className="text-3xl font-bold mb-6">Launch GPU Instance</h1>

      {/* Model Selection */}
      <div className="bg-white shadow rounded-lg p-6 mb-6">
        <h2 className="text-xl font-semibold mb-4">1. Select Model</h2>
        <div className="space-y-2">
          {models.map((model) => (
            <div
              key={model.id}
              className={`p-4 border rounded cursor-pointer hover:bg-gray-50 ${
                selectedModel === model.name ? 'border-blue-500 bg-blue-50' : ''
              }`}
              onClick={() => setSelectedModel(model.name)}
            >
              <div className="flex justify-between items-start">
                <div>
                  <h3 className="font-medium">{model.name}</h3>
                  <p className="text-sm text-gray-600">
                    {model.family} • {model.size} • {model.type}
                  </p>
                </div>
                <span className="text-sm text-gray-500">
                  {model.vram_required_gb}GB VRAM
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Provider & Configuration */}
      {selectedModel && (
        <div className="bg-white shadow rounded-lg p-6 mb-6">
          <h2 className="text-xl font-semibold mb-4">2. Configure Instance</h2>
          
          {/* Provider */}
          <div className="mb-4">
            <label className="block text-sm font-medium mb-2">Cloud Provider</label>
            <select
              className="w-full border rounded p-2"
              value={config.provider}
              onChange={(e) =>
                setConfig({
                  ...config,
                  provider: e.target.value,
                  region: providerOptions[e.target.value as keyof typeof providerOptions].regions[0],
                  instance_type:
                    providerOptions[e.target.value as keyof typeof providerOptions].instances[0],
                })
              }
            >
              <option value="azure">Azure</option>
              <option value="aws">AWS</option>
              <option value="gcp">GCP</option>
            </select>
          </div>

          {/* Region */}
          <div className="mb-4">
            <label className="block text-sm font-medium mb-2">Region</label>
            <select
              className="w-full border rounded p-2"
              value={config.region}
              onChange={(e) => setConfig({ ...config, region: e.target.value })}
            >
              {providerOptions[config.provider as keyof typeof providerOptions].regions.map((region) => (
                <option key={region} value={region}>
                  {region}
                </option>
              ))}
            </select>
          </div>

          {/* Instance Type */}
          <div className="mb-4">
            <label className="block text-sm font-medium mb-2">Instance Type</label>
            <select
              className="w-full border rounded p-2"
              value={config.instance_type}
              onChange={(e) => setConfig({ ...config, instance_type: e.target.value })}
            >
              {providerOptions[config.provider as keyof typeof providerOptions].instances.map(
                (instance) => (
                  <option key={instance} value={instance}>
                    {instance}
                  </option>
                )
              )}
            </select>
          </div>

          {/* Spot Instance */}
          <div className="mb-4">
            <label className="flex items-center">
              <input
                type="checkbox"
                checked={config.use_spot}
                onChange={(e) => setConfig({ ...config, use_spot: e.target.checked })}
                className="mr-2"
              />
              <span className="text-sm">Use Spot Instance (70-90% cost savings)</span>
            </label>
          </div>

          {/* Launch Button */}
          <button
            onClick={handleLaunch}
            disabled={launching || !selectedModel}
            className="w-full bg-blue-600 text-white py-3 rounded hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed font-medium"
          >
            {launching ? 'Launching...' : 'Launch Instance'}
          </button>
        </div>
      )}

      {/* Status */}
      {status && (
        <div className="bg-white shadow rounded-lg p-6">
          <h2 className="text-xl font-semibold mb-4">Launch Status</h2>
          <div className="space-y-2">
            <div className="flex justify-between">
              <span>Job ID:</span>
              <span className="font-mono text-sm">{jobId}</span>
            </div>
            <div className="flex justify-between">
              <span>Status:</span>
              <span
                className={`font-medium ${
                  status.status === 'completed'
                    ? 'text-green-600'
                    : status.status === 'failed'
                    ? 'text-red-600'
                    : 'text-blue-600'
                }`}
              >
                {status.status}
              </span>
            </div>
            {status.progress && (
              <div>
                <div className="flex justify-between mb-1">
                  <span>Progress:</span>
                  <span>{status.progress}%</span>
                </div>
                <div className="w-full bg-gray-200 rounded h-2">
                  <div
                    className="bg-blue-600 h-2 rounded transition-all"
                    style={{ width: `${status.progress}%` }}
                  ></div>
                </div>
              </div>
            )}
            {status.stages && (
              <div className="mt-4">
                <h3 className="font-medium mb-2">Stages:</h3>
                <div className="space-y-1 text-sm font-mono">
                  {status.stages.map((stage: string, idx: number) => (
                    <div key={idx}>{stage}</div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

