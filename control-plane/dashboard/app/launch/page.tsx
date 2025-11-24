'use client';

import { useState, useEffect, useMemo } from 'react';
import { 
  Search, 
  Filter, 
  Cpu, 
  Zap, 
  Globe, 
  Server, 
  CheckCircle2, 
  AlertCircle, 
  ArrowRight, 
  ArrowLeft,
  ChevronRight,
  Box,
  MemoryStick,
  LayoutGrid,
  DollarSign,
  Cloud
} from 'lucide-react';

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

interface InstanceSpec {
  vcpu: number;
  memory_gb: number;
  gpu_count: number;
  gpu_vram_gb: number;
  gpu_model?: string;
}

// Azure GPU instance specifications
const azureInstanceSpecs: Record<string, InstanceSpec> = {
  // NVadsA10_v5 Series (NVIDIA A10 GPUs)
  'Standard_NV12ads_A10_v5': { vcpu: 12, memory_gb: 110, gpu_count: 1, gpu_vram_gb: 8, gpu_model: 'NVIDIA A10' },
  'Standard_NV36ads_A10_v5': { vcpu: 36, memory_gb: 440, gpu_count: 1, gpu_vram_gb: 24, gpu_model: 'NVIDIA A10' },
  'Standard_NV72ads_A10_v5': { vcpu: 72, memory_gb: 880, gpu_count: 2, gpu_vram_gb: 48, gpu_model: 'NVIDIA A10' },
  
  // NCv3 Series (NVIDIA Tesla V100)
  'Standard_NC6s_v3': { vcpu: 6, memory_gb: 112, gpu_count: 1, gpu_vram_gb: 16, gpu_model: 'NVIDIA Tesla V100' },
  'Standard_NC12s_v3': { vcpu: 12, memory_gb: 224, gpu_count: 2, gpu_vram_gb: 32, gpu_model: 'NVIDIA Tesla V100' },
  'Standard_NC24s_v3': { vcpu: 24, memory_gb: 448, gpu_count: 4, gpu_vram_gb: 64, gpu_model: 'NVIDIA Tesla V100' },
  'Standard_NC24rs_v3': { vcpu: 24, memory_gb: 448, gpu_count: 4, gpu_vram_gb: 64, gpu_model: 'NVIDIA Tesla V100' },
  
  // NCasT4_v3 Series (NVIDIA T4)
  'Standard_NC4as_T4_v3': { vcpu: 4, memory_gb: 28, gpu_count: 1, gpu_vram_gb: 16, gpu_model: 'NVIDIA T4' },
  'Standard_NC8as_T4_v3': { vcpu: 8, memory_gb: 56, gpu_count: 1, gpu_vram_gb: 16, gpu_model: 'NVIDIA T4' },
  'Standard_NC16as_T4_v3': { vcpu: 16, memory_gb: 110, gpu_count: 1, gpu_vram_gb: 16, gpu_model: 'NVIDIA T4' },
  'Standard_NC64as_T4_v3': { vcpu: 64, memory_gb: 440, gpu_count: 4, gpu_vram_gb: 64, gpu_model: 'NVIDIA T4' },
  
  // NC_A100_v4 Series (NVIDIA A100 PCIe)
  'Standard_NC96ads_A100_v4': { vcpu: 96, memory_gb: 880, gpu_count: 4, gpu_vram_gb: 320, gpu_model: 'NVIDIA A100 PCIe' },
  
  // NCads_H100_v5 Series (NVIDIA H100)
  'Standard_NC48ads_H100_v5': { vcpu: 48, memory_gb: 880, gpu_count: 2, gpu_vram_gb: 160, gpu_model: 'NVIDIA H100' },
  'Standard_NC96ads_H100_v5': { vcpu: 96, memory_gb: 1760, gpu_count: 4, gpu_vram_gb: 320, gpu_model: 'NVIDIA H100' },
  
  // ND Series (NVIDIA Tesla P40)
  'Standard_ND6s': { vcpu: 6, memory_gb: 112, gpu_count: 1, gpu_vram_gb: 24, gpu_model: 'NVIDIA Tesla P40' },
  'Standard_ND12s': { vcpu: 12, memory_gb: 224, gpu_count: 2, gpu_vram_gb: 48, gpu_model: 'NVIDIA Tesla P40' },
  'Standard_ND24s': { vcpu: 24, memory_gb: 448, gpu_count: 4, gpu_vram_gb: 96, gpu_model: 'NVIDIA Tesla P40' },
  'Standard_ND24rs': { vcpu: 24, memory_gb: 448, gpu_count: 4, gpu_vram_gb: 96, gpu_model: 'NVIDIA Tesla P40' },
  
  // NDv2 Series (NVIDIA Tesla V100)
  'Standard_ND40s_v2': { vcpu: 40, memory_gb: 672, gpu_count: 8, gpu_vram_gb: 128, gpu_model: 'NVIDIA Tesla V100' },
  
  // NDasrA100_v4 Series (NVIDIA A100 40GB)
  'Standard_ND96asr_v4': { vcpu: 96, memory_gb: 900, gpu_count: 8, gpu_vram_gb: 320, gpu_model: 'NVIDIA A100 40GB' },
  'Standard_ND96amsr_A100_v4': { vcpu: 96, memory_gb: 1920, gpu_count: 8, gpu_vram_gb: 320, gpu_model: 'NVIDIA A100 40GB' },
  
  // NVv3 Series (NVIDIA Tesla M60)
  'Standard_NV12s_v3': { vcpu: 12, memory_gb: 112, gpu_count: 2, gpu_vram_gb: 16, gpu_model: 'NVIDIA Tesla M60' },
  'Standard_NV24s_v3': { vcpu: 24, memory_gb: 224, gpu_count: 4, gpu_vram_gb: 32, gpu_model: 'NVIDIA Tesla M60' },
  'Standard_NV48s_v3': { vcpu: 48, memory_gb: 448, gpu_count: 8, gpu_vram_gb: 64, gpu_model: 'NVIDIA Tesla M60' },
  
  // NCv2 Series (NVIDIA Tesla P100)
  'Standard_NC6s_v2': { vcpu: 6, memory_gb: 112, gpu_count: 1, gpu_vram_gb: 16, gpu_model: 'NVIDIA Tesla P100' },
  'Standard_NC12s_v2': { vcpu: 12, memory_gb: 224, gpu_count: 2, gpu_vram_gb: 32, gpu_model: 'NVIDIA Tesla P100' },
  'Standard_NC24s_v2': { vcpu: 24, memory_gb: 448, gpu_count: 4, gpu_vram_gb: 64, gpu_model: 'NVIDIA Tesla P100' },
  'Standard_NC24rs_v2': { vcpu: 24, memory_gb: 448, gpu_count: 4, gpu_vram_gb: 64, gpu_model: 'NVIDIA Tesla P100' },
  
  // NC Series (NVIDIA Tesla K80)
  'Standard_NC6': { vcpu: 6, memory_gb: 56, gpu_count: 1, gpu_vram_gb: 12, gpu_model: 'NVIDIA Tesla K80' },
  'Standard_NC12': { vcpu: 12, memory_gb: 112, gpu_count: 2, gpu_vram_gb: 24, gpu_model: 'NVIDIA Tesla K80' },
  'Standard_NC24': { vcpu: 24, memory_gb: 224, gpu_count: 4, gpu_vram_gb: 48, gpu_model: 'NVIDIA Tesla K80' },
  'Standard_NC24r': { vcpu: 24, memory_gb: 224, gpu_count: 4, gpu_vram_gb: 48, gpu_model: 'NVIDIA Tesla K80' },
  
  // NV Series (NVIDIA Tesla M60)
  'Standard_NV6': { vcpu: 6, memory_gb: 56, gpu_count: 1, gpu_vram_gb: 8, gpu_model: 'NVIDIA Tesla M60' },
  'Standard_NV12': { vcpu: 12, memory_gb: 112, gpu_count: 2, gpu_vram_gb: 16, gpu_model: 'NVIDIA Tesla M60' },
  'Standard_NV24': { vcpu: 24, memory_gb: 224, gpu_count: 4, gpu_vram_gb: 32, gpu_model: 'NVIDIA Tesla M60' },
};

const providerOptions = {
  azure: {
    regions: [
      'eastus',
      'westus2',
      'centralus',
      'centralindia',
      'southindia',
      'westindia',
    ],
    instances: [
      'Standard_NV12ads_A10_v5',
      'Standard_NV36ads_A10_v5',
      'Standard_NV72ads_A10_v5',
      'Standard_NC6s_v3',
      'Standard_NC12s_v3',
      'Standard_NC24s_v3',
      'Standard_NC24rs_v3',
      'Standard_NC4as_T4_v3',
      'Standard_NC8as_T4_v3',
      'Standard_NC16as_T4_v3',
      'Standard_NC64as_T4_v3',
      'Standard_NC96ads_A100_v4',
      'Standard_NC48ads_H100_v5',
      'Standard_NC96ads_H100_v5',
      'Standard_ND6s',
      'Standard_ND12s',
      'Standard_ND24s',
      'Standard_ND24rs',
      'Standard_ND40s_v2',
      'Standard_ND96asr_v4',
      'Standard_ND96amsr_A100_v4',
      'Standard_NV12s_v3',
      'Standard_NV24s_v3',
      'Standard_NV48s_v3',
      'Standard_NC6s_v2',
      'Standard_NC12s_v2',
      'Standard_NC24s_v2',
      'Standard_NC24rs_v2',
      'Standard_NC6',
      'Standard_NC12',
      'Standard_NC24',
      'Standard_NC24r',
      'Standard_NV6',
      'Standard_NV12',
      'Standard_NV24',
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

export default function LaunchPage() {
  // State management
  const [step, setStep] = useState(1);
  const [models, setModels] = useState<Model[]>([]);
  const [selectedModel, setSelectedModel] = useState<string>('');
  const [config, setConfig] = useState<LaunchConfig>({
    model_name: '',
    provider: 'azure',
    region: 'eastus',
    instance_type: '', // Empty initially
    use_spot: true,
  });
  const [launching, setLaunching] = useState(false);
  const [status, setStatus] = useState<any>(null);
  const [jobId, setJobId] = useState<string>('');

  // Filters for instance table
  const [searchQuery, setSearchQuery] = useState('');
  const [gpuModelFilter, setGpuModelFilter] = useState<string>('all');
  const [minVramFilter, setMinVramFilter] = useState<number>(0);

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
    if (!config.instance_type) return;
    
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
    }, 3000);
  };

  // Get unique GPU models for filter
  const uniqueGpuModels = useMemo(() => {
    if (config.provider !== 'azure') return [];
    const models = new Set<string>();
    providerOptions.azure.instances.forEach(instance => {
      const spec = azureInstanceSpecs[instance];
      if (spec?.gpu_model) {
        models.add(spec.gpu_model);
      }
    });
    return Array.from(models).sort();
  }, [config.provider]);

  // Filter instances
  const filteredInstances = useMemo(() => {
    if (config.provider !== 'azure') {
      return providerOptions[config.provider as keyof typeof providerOptions].instances;
    }

    return providerOptions.azure.instances.filter(instance => {
      const spec = azureInstanceSpecs[instance];
      if (!spec) return false;

      // Search filter
      if (searchQuery && !instance.toLowerCase().includes(searchQuery.toLowerCase())) {
        return false;
      }

      // GPU model filter
      if (gpuModelFilter !== 'all' && spec.gpu_model !== gpuModelFilter) {
        return false;
      }

      // Min VRAM filter
      if (spec.gpu_vram_gb < minVramFilter) {
        return false;
      }

      return true;
    });
  }, [config.provider, searchQuery, gpuModelFilter, minVramFilter]);

  // Get selected model VRAM requirement
  const selectedModelVram = useMemo(() => {
    const model = models.find(m => m.name === selectedModel);
    return model?.vram_required_gb || 0;
  }, [models, selectedModel]);

  // Navigation handlers
  const nextStep = () => {
    if (step === 1 && selectedModel) setStep(2);
    if (step === 2 && config.region) setStep(3);
  };

  const prevStep = () => {
    if (step > 1) setStep(step - 1);
  };

  return (
    <div className="max-w-6xl mx-auto p-6 min-h-screen">
      {/* Header & Stepper */}
      <div className="mb-10">
        <h1 className="text-3xl font-bold text-gray-900 mb-8">Launch GPU Instance</h1>
        
        {/* Stepper */}
        <div className="relative">
          <div className="absolute left-0 top-1/2 transform -translate-y-1/2 w-full h-1 bg-gray-100 -z-10 rounded-full"></div>
          <div className="flex justify-between max-w-3xl mx-auto px-4">
            {/* Step 1 */}
            <div className={`flex flex-col items-center bg-white px-4 z-10 transition-colors duration-300`}>
              <div className={`w-10 h-10 rounded-full flex items-center justify-center text-sm font-bold mb-2 shadow-sm transition-all duration-300 ${
                step >= 1 ? 'bg-blue-600 text-white scale-110' : 'bg-gray-100 text-gray-400 border border-gray-200'
              }`}>
                {step > 1 ? <CheckCircle2 className="w-6 h-6" /> : '1'}
              </div>
              <span className={`text-sm font-medium ${step >= 1 ? 'text-blue-600' : 'text-gray-400'}`}>Select Model</span>
            </div>

            {/* Step 2 */}
            <div className={`flex flex-col items-center bg-white px-4 z-10 transition-colors duration-300`}>
              <div className={`w-10 h-10 rounded-full flex items-center justify-center text-sm font-bold mb-2 shadow-sm transition-all duration-300 ${
                step >= 2 ? 'bg-blue-600 text-white scale-110' : 'bg-gray-100 text-gray-400 border border-gray-200'
              }`}>
                 {step > 2 ? <CheckCircle2 className="w-6 h-6" /> : '2'}
              </div>
              <span className={`text-sm font-medium ${step >= 2 ? 'text-blue-600' : 'text-gray-400'}`}>Configuration</span>
            </div>

            {/* Step 3 */}
            <div className={`flex flex-col items-center bg-white px-4 z-10 transition-colors duration-300`}>
              <div className={`w-10 h-10 rounded-full flex items-center justify-center text-sm font-bold mb-2 shadow-sm transition-all duration-300 ${
                step >= 3 ? 'bg-blue-600 text-white scale-110' : 'bg-gray-100 text-gray-400 border border-gray-200'
              }`}>
                3
              </div>
              <span className={`text-sm font-medium ${step >= 3 ? 'text-blue-600' : 'text-gray-400'}`}>Instance Type</span>
            </div>
          </div>
        </div>
      </div>

      {/* Step Content */}
      <div className="bg-white shadow-xl shadow-slate-200/50 rounded-2xl border border-gray-100 overflow-hidden min-h-[500px] flex flex-col">
        
        {/* Step 1: Select Model */}
        {step === 1 && (
          <div className="p-8 flex-1 animate-in fade-in slide-in-from-bottom-4 duration-500">
            <div className="flex items-center justify-between mb-6">
              <div>
                <h2 className="text-2xl font-bold text-gray-900">Select a Model</h2>
                <p className="text-gray-500 mt-1">Choose the AI model you want to deploy</p>
              </div>
              <div className="bg-blue-50 text-blue-700 px-4 py-2 rounded-lg text-sm font-medium flex items-center">
                <Box className="w-4 h-4 mr-2" />
                {models.length} Models Available
              </div>
            </div>

            {models.length === 0 ? (
              <div className="text-center py-20 bg-gray-50 rounded-xl border-2 border-dashed border-gray-200">
                <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto mb-4"></div>
                <p className="text-gray-500 font-medium">Fetching latest models...</p>
              </div>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
                {models.map((model) => (
                  <div
                    key={model.id}
                    onClick={() => setSelectedModel(model.name)}
                    className={`group relative p-6 border-2 rounded-xl cursor-pointer transition-all duration-200 ${
                      selectedModel === model.name
                        ? 'border-blue-600 bg-blue-50/50 ring-4 ring-blue-100 shadow-inner'
                        : 'border-gray-200 hover:border-blue-400 hover:bg-slate-50 hover:shadow-md'
                    }`}
                  >
                    <div className="flex justify-between items-start mb-3">
                      <div>
                        <h3 className="font-bold text-lg text-gray-900 group-hover:text-blue-700 transition-colors">{model.name}</h3>
                        <div className="flex items-center gap-2 mt-1.5">
                          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800">
                            {model.family}
                          </span>
                          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800">
                            {model.size}
                          </span>
                        </div>
                      </div>
                      {selectedModel === model.name && (
                        <div className="bg-blue-600 text-white p-1.5 rounded-full shadow-sm">
                          <CheckCircle2 className="w-5 h-5" />
                        </div>
                      )}
                    </div>
                    
                    <div className="mt-4 pt-4 border-t border-gray-100 flex items-center justify-between">
                      <div className="flex items-center text-sm font-medium text-gray-600">
                         <MemoryStick className="w-4 h-4 mr-2 text-gray-400" />
                         VRAM Required
                      </div>
                      <div className={`flex items-center font-bold text-sm px-3 py-1 rounded-full ${
                        model.vram_required_gb > 40 ? 'bg-amber-100 text-amber-800' : 'bg-green-100 text-green-800'
                      }`}>
                        {model.vram_required_gb} GB
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {/* Step 2: Configuration */}
        {step === 2 && (
          <div className="p-8 flex-1 animate-in fade-in slide-in-from-right-4 duration-500">
            <div className="max-w-3xl mx-auto">
              <div className="mb-8 text-center">
                 <h2 className="text-2xl font-bold text-gray-900">Configure Environment</h2>
                 <p className="text-gray-500 mt-1">Choose where to run your model</p>
              </div>
              
              <div className="space-y-8">
                {/* Provider Selection */}
                <div className="bg-white p-6 rounded-xl border border-gray-200 shadow-sm">
                  <label className="block text-sm font-bold text-gray-900 mb-4 flex items-center">
                    <Cloud className="w-4 h-4 mr-2 text-blue-600" />
                    Cloud Provider
                  </label>
                  <div className="grid grid-cols-3 gap-4">
                    {['azure', 'aws', 'gcp'].map((provider) => (
                      <div
                        key={provider}
                        onClick={() => setConfig({
                          ...config,
                          provider,
                          region: providerOptions[provider as keyof typeof providerOptions].regions[0],
                          instance_type: providerOptions[provider as keyof typeof providerOptions].instances[0],
                        })}
                        className={`cursor-pointer relative p-4 border-2 rounded-xl text-center transition-all hover:shadow-md ${
                          config.provider === provider
                            ? 'border-blue-600 bg-blue-50/50 text-blue-700 ring-2 ring-blue-100'
                            : 'border-gray-100 hover:border-blue-300 text-gray-600 bg-gray-50/50'
                        }`}
                      >
                        {/* Simple Logo Placeholders */}
                        <div className="h-10 flex items-center justify-center mb-2 text-2xl">
                           {provider === 'azure' ? '🔷' : provider === 'aws' ? '🔶' : '🌈'}
                        </div>
                        <div className="font-bold capitalize">{provider === 'gcp' ? 'GCP' : provider === 'aws' ? 'AWS' : 'Azure'}</div>
                        {config.provider === provider && (
                          <div className="absolute top-2 right-2 text-blue-600">
                            <CheckCircle2 className="w-4 h-4" />
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                </div>

                {/* Region Selection */}
                <div className="bg-white p-6 rounded-xl border border-gray-200 shadow-sm">
                  <label className="block text-sm font-bold text-gray-900 mb-4 flex items-center">
                    <Globe className="w-4 h-4 mr-2 text-blue-600" />
                    Region <span className="text-gray-400 font-normal ml-2 text-xs">({providerOptions[config.provider as keyof typeof providerOptions].regions.length} available)</span>
                  </label>
                  <div className="relative">
                    <select
                      className="w-full appearance-none border-2 border-gray-200 rounded-xl px-4 py-3.5 text-base focus:outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-50 transition-all bg-white hover:border-gray-300 cursor-pointer"
                      value={config.region}
                      onChange={(e) => setConfig({ ...config, region: e.target.value })}
                    >
                      {providerOptions[config.provider as keyof typeof providerOptions].regions.map((region) => (
                        <option key={region} value={region}>
                          {region}
                        </option>
                      ))}
                    </select>
                    <div className="absolute right-4 top-1/2 transform -translate-y-1/2 pointer-events-none text-gray-500">
                      <ChevronRight className="w-5 h-5 rotate-90" />
                    </div>
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {providerOptions[config.provider as keyof typeof providerOptions].regions.map(r => (
                       <span key={r} className={`text-xs px-2 py-1 rounded-md border ${
                         config.region === r ? 'bg-blue-100 border-blue-200 text-blue-800' : 'bg-gray-50 border-gray-100 text-gray-500'
                       }`}>
                         {r}
                       </span>
                    ))}
                  </div>
                </div>

                {/* Spot Instance Toggle */}
                <div 
                  className={`p-6 rounded-xl border-2 transition-all cursor-pointer flex items-center justify-between ${
                    config.use_spot 
                      ? 'bg-green-50/50 border-green-200 shadow-sm' 
                      : 'bg-white border-gray-200 hover:border-gray-300'
                  }`}
                  onClick={() => setConfig({ ...config, use_spot: !config.use_spot })}
                >
                  <div className="flex items-center">
                    <div className={`w-12 h-7 rounded-full p-1 transition-colors mr-5 duration-300 ${config.use_spot ? 'bg-green-500' : 'bg-gray-200'}`}>
                      <div className={`bg-white w-5 h-5 rounded-full shadow-sm transform transition-transform duration-300 ${config.use_spot ? 'translate-x-5' : ''}`}></div>
                    </div>
                    <div>
                      <span className="font-bold text-gray-900 flex items-center gap-2">
                        Use Spot Instance
                        <span className="bg-green-100 text-green-800 text-xs px-2 py-0.5 rounded-full">Recommended</span>
                      </span>
                      <span className="text-sm text-gray-500 block mt-0.5">Save 70-90% on compute costs. Instance may be reclaimed.</span>
                    </div>
                  </div>
                  <div className="h-12 w-12 rounded-full bg-green-100 flex items-center justify-center text-green-600">
                    <DollarSign className="w-6 h-6" />
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Step 3: Instance Selection */}
        {step === 3 && (
          <div className="p-8 flex-1 flex flex-col animate-in fade-in slide-in-from-right-4 duration-500">
            <div className="flex justify-between items-start mb-6">
              <div>
                <h2 className="text-2xl font-bold text-gray-900">Select Instance Type</h2>
                <p className="text-gray-500 mt-1 flex items-center gap-2">
                  <Globe className="w-4 h-4" /> {config.region} 
                  <span className="text-gray-300">|</span>
                  <Cloud className="w-4 h-4" /> {config.provider}
                </p>
              </div>
              
              {/* Selection Summary Card */}
              <div className="hidden lg:block bg-slate-50 px-5 py-3 rounded-lg border border-gray-200 text-sm">
                <div className="font-semibold text-gray-900 mb-2">Deployment Summary</div>
                <div className="flex gap-6">
                  <div>
                    <div className="text-xs text-gray-500 uppercase tracking-wider font-semibold">Model</div>
                    <div className="font-medium text-blue-700">{selectedModel}</div>
                  </div>
                  <div>
                    <div className="text-xs text-gray-500 uppercase tracking-wider font-semibold">VRAM Needed</div>
                    <div className="font-medium text-gray-900">{selectedModelVram} GB</div>
                  </div>
                </div>
              </div>
            </div>

            {/* Filters */}
            <div className="bg-white p-5 rounded-xl border border-gray-200 shadow-sm mb-6 grid grid-cols-1 md:grid-cols-12 gap-4">
              <div className="md:col-span-5">
                <label className="block text-xs font-bold text-gray-500 mb-1.5 uppercase tracking-wider">Search Instance</label>
                <div className="relative">
                  <input
                    type="text"
                    placeholder="e.g. Standard_NC24..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    className="w-full border border-gray-300 rounded-lg pl-10 pr-3 py-2.5 text-sm focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition-all"
                  />
                  <Search className="w-4 h-4 absolute left-3.5 top-3 text-gray-400" />
                </div>
              </div>
              <div className="md:col-span-4">
                <label className="block text-xs font-bold text-gray-500 mb-1.5 uppercase tracking-wider">GPU Model</label>
                <div className="relative">
                  <select
                    value={gpuModelFilter}
                    onChange={(e) => setGpuModelFilter(e.target.value)}
                    className="w-full border border-gray-300 rounded-lg pl-3 pr-10 py-2.5 text-sm focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 appearance-none bg-white transition-all"
                  >
                    <option value="all">All GPU Models</option>
                    {uniqueGpuModels.map(model => (
                      <option key={model} value={model}>{model}</option>
                    ))}
                  </select>
                  <Filter className="w-4 h-4 absolute right-3.5 top-3 text-gray-400 pointer-events-none" />
                </div>
              </div>
              <div className="md:col-span-3">
                <label className="block text-xs font-bold text-gray-500 mb-1.5 uppercase tracking-wider">Min VRAM</label>
                <div className="relative">
                  <input
                    type="number"
                    value={minVramFilter}
                    onChange={(e) => setMinVramFilter(parseInt(e.target.value) || 0)}
                    className="w-full border border-gray-300 rounded-lg pl-3 pr-12 py-2.5 text-sm focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition-all"
                  />
                  <span className="absolute right-3.5 top-2.5 text-xs text-gray-500 font-bold bg-gray-100 px-1.5 py-0.5 rounded">GB</span>
                </div>
              </div>
            </div>

            {/* Instance List */}
            <div className="flex-1 border border-gray-200 rounded-xl overflow-hidden flex flex-col bg-white shadow-sm">
              <div className="overflow-y-auto flex-1">
                <table className="w-full text-sm text-left">
                  <thead className="bg-gray-50/80 backdrop-blur-sm sticky top-0 z-10 text-xs uppercase font-bold text-gray-500 tracking-wider border-b border-gray-200">
                    <tr>
                      <th className="px-6 py-4">Instance Name</th>
                      <th className="px-6 py-4">GPU Model</th>
                      <th className="px-6 py-4 text-right">vCPU</th>
                      <th className="px-6 py-4 text-right">Memory</th>
                      <th className="px-6 py-4 text-right">Count</th>
                      <th className="px-6 py-4 text-right">VRAM (Total)</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {filteredInstances.length === 0 ? (
                      <tr>
                         <td colSpan={6} className="px-6 py-12 text-center text-gray-500">
                           <div className="mx-auto w-12 h-12 bg-gray-100 rounded-full flex items-center justify-center mb-3">
                             <Search className="w-6 h-6 text-gray-400" />
                           </div>
                           <p className="font-medium">No instances found</p>
                           <p className="text-xs mt-1">Try adjusting your filters</p>
                         </td>
                      </tr>
                    ) : (
                      filteredInstances.map((instance) => {
                        const spec = azureInstanceSpecs[instance];
                        const isSelected = config.instance_type === instance;
                        const meetsVramRequirement = selectedModelVram === 0 || (spec?.gpu_vram_gb || 0) >= selectedModelVram;
                        
                        if (!spec) return null;

                        return (
                          <tr
                            key={instance}
                            onClick={() => meetsVramRequirement && setConfig({ ...config, instance_type: instance })}
                            className={`transition-all duration-150 group ${
                              isSelected ? 'bg-blue-50/70' : 'hover:bg-gray-50'
                            } ${!meetsVramRequirement ? 'opacity-50 grayscale-[0.5] cursor-not-allowed bg-slate-50' : 'cursor-pointer'}`}
                          >
                            <td className="px-6 py-4">
                              <div className="flex items-center">
                                <div className={`w-5 h-5 rounded-full border-2 mr-4 flex items-center justify-center transition-all ${
                                  isSelected ? 'border-blue-600 bg-blue-600' : 'border-gray-300 group-hover:border-blue-400'
                                }`}>
                                  {isSelected && <div className="w-2 h-2 bg-white rounded-full shadow-sm"></div>}
                                </div>
                                <div>
                                  <div className={`font-semibold ${isSelected ? 'text-blue-900' : 'text-gray-900'}`}>{instance}</div>
                                  {!meetsVramRequirement && (
                                    <div className="text-xs text-red-500 font-bold mt-1 flex items-center">
                                      <AlertCircle className="w-3 h-3 mr-1" />
                                      Needs {selectedModelVram}GB VRAM
                                    </div>
                                  )}
                                </div>
                              </div>
                            </td>
                            <td className="px-6 py-4 text-gray-600 font-medium">{spec.gpu_model}</td>
                            <td className="px-6 py-4 text-right font-mono text-gray-600">{spec.vcpu}</td>
                            <td className="px-6 py-4 text-right font-mono text-gray-600">{spec.memory_gb} GB</td>
                            <td className="px-6 py-4 text-right font-mono text-gray-600">{spec.gpu_count}</td>
                            <td className="px-6 py-4 text-right">
                              <span className={`font-mono font-bold text-sm px-2.5 py-1 rounded-lg ${
                                meetsVramRequirement 
                                  ? 'bg-blue-100 text-blue-700' 
                                  : 'bg-red-100 text-red-700'
                              }`}>
                                {spec.gpu_vram_gb} GB
                              </span>
                            </td>
                          </tr>
                        );
                      })
                    )}
                  </tbody>
                </table>
              </div>
              <div className="bg-gray-50 px-6 py-3 border-t border-gray-200 text-xs text-gray-500 flex justify-between items-center">
                <span>Showing {filteredInstances.length} instances</span>
                {config.use_spot && <span className="flex items-center text-green-600 font-medium"><Zap className="w-3 h-3 mr-1" /> Spot pricing active</span>}
              </div>
            </div>
          </div>
        )}

        {/* Footer Navigation */}
        <div className="bg-white px-8 py-6 border-t border-gray-100 flex justify-between items-center mt-auto">
          <button
            onClick={prevStep}
            disabled={step === 1 || launching}
            className={`px-6 py-3 rounded-xl font-bold text-sm transition-all flex items-center ${
              step === 1 || launching
                ? 'text-gray-300 cursor-not-allowed'
                : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900'
            }`}
          >
            <ArrowLeft className="w-4 h-4 mr-2" />
            Back
          </button>

          {step < 3 ? (
            <button
              onClick={nextStep}
              disabled={!selectedModel}
              className="px-8 py-3 bg-blue-600 text-white rounded-xl font-bold text-sm hover:bg-blue-700 transition-all shadow-lg shadow-blue-200 hover:shadow-blue-300 hover:-translate-y-0.5 disabled:opacity-50 disabled:shadow-none disabled:cursor-not-allowed disabled:hover:translate-y-0 flex items-center"
            >
              Next Step
              <ArrowRight className="w-4 h-4 ml-2" />
            </button>
          ) : (
            <div className="flex items-center gap-4">
              {status && (
                <div className="text-sm px-4 py-2 bg-gray-50 rounded-lg border border-gray-100">
                  <span className={`font-bold flex items-center ${
                    status.status === 'failed' ? 'text-red-600' : 'text-blue-600'
                  }`}>
                    {status.status === 'failed' ? <AlertCircle className="w-4 h-4 mr-2" /> : <Server className="w-4 h-4 mr-2" />}
                    {status.status === 'failed' ? 'Launch Failed' : status.status === 'completed' ? 'Success!' : 'Launching...'}
                  </span>
                </div>
              )}
              <button
                onClick={handleLaunch}
                disabled={launching || !config.instance_type}
                className="px-8 py-3 bg-green-600 text-white rounded-xl font-bold text-sm hover:bg-green-700 transition-all shadow-lg shadow-green-200 hover:shadow-green-300 hover:-translate-y-0.5 disabled:opacity-50 disabled:shadow-none disabled:cursor-not-allowed disabled:hover:translate-y-0 flex items-center"
              >
                {launching ? (
                  <>
                    <svg className="animate-spin -ml-1 mr-3 h-5 w-5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                    Deploying...
                  </>
                ) : (
                  <>
                    Launch Instance
                    <Zap className="w-4 h-4 ml-2 fill-current" />
                  </>
                )}
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Detailed Status (only show when launching) */}
      {launching && status && (
        <div className="mt-8 bg-white shadow-xl rounded-2xl p-8 border border-gray-100 animate-in fade-in slide-in-from-bottom-4">
          <div className="flex items-center justify-between mb-6">
            <h3 className="text-lg font-bold text-gray-900 flex items-center">
              <Server className="w-5 h-5 mr-2 text-blue-600" />
              Deployment Status
            </h3>
            <span className="font-mono text-xs text-gray-400 bg-gray-50 px-2 py-1 rounded">Job ID: {jobId}</span>
          </div>
          
          {/* Progress Bar */}
            {status.progress && (
            <div className="mb-8">
              <div className="flex justify-between mb-2">
                <span className="text-sm font-medium text-gray-600">Overall Progress</span>
                <span className="text-sm font-bold text-blue-600">{status.progress}%</span>
                </div>
              <div className="w-full bg-gray-100 rounded-full h-3 overflow-hidden">
                  <div
                  className="bg-blue-600 h-3 rounded-full transition-all duration-700 ease-out relative overflow-hidden"
                    style={{ width: `${status.progress}%` }}
                  >
                    <div className="absolute inset-0 bg-white/30 w-full h-full animate-pulse"></div>
                  </div>
                </div>
              </div>
            )}

          {/* Logs / Stages */}
            {status.stages && (
            <div className="bg-slate-900 rounded-xl p-6 font-mono text-sm text-green-400 h-64 overflow-y-auto shadow-inner border border-slate-800">
                  {status.stages.map((stage: string, idx: number) => (
                <div key={idx} className="mb-2 flex">
                  <span className="text-slate-500 mr-3 select-none">[{new Date().toLocaleTimeString()}]</span>
                  <span>{stage}</span>
                </div>
              ))}
              <div className="animate-pulse text-blue-400">_</div>
              </div>
            )}
        </div>
      )}
    </div>
  );
}