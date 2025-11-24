-- Regions and Instance Types Schema
-- This schema stores cloud provider regions and their available GPU instance types

-- Cloud Regions Table
CREATE TABLE IF NOT EXISTS regions (
    id SERIAL PRIMARY KEY,
    provider VARCHAR(50) NOT NULL, -- 'azure', 'aws', 'gcp'
    region_code VARCHAR(50) NOT NULL, -- e.g., 'eastus', 'us-west-2', 'us-central1'
    region_name VARCHAR(100) NOT NULL, -- e.g., 'East US', 'US West (Oregon)', 'Iowa'
    location VARCHAR(100), -- e.g., 'Virginia, US', 'Oregon, US'
    is_available BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(provider, region_code)
);

-- GPU Instance Types Table
CREATE TABLE IF NOT EXISTS instance_types (
    id SERIAL PRIMARY KEY,
    provider VARCHAR(50) NOT NULL, -- 'azure', 'aws', 'gcp'
    instance_type VARCHAR(100) NOT NULL, -- e.g., 'Standard_NC4as_T4_v3', 'p3.2xlarge', 'n1-standard-4'
    instance_name VARCHAR(200), -- Human-readable name

    -- Compute Specs
    vcpu_count INTEGER NOT NULL,
    memory_gb DECIMAL(10, 2) NOT NULL,

    -- GPU Specs
    gpu_count INTEGER NOT NULL,
    gpu_memory_gb DECIMAL(10, 2) NOT NULL,
    gpu_model VARCHAR(100) NOT NULL, -- e.g., 'NVIDIA T4', 'NVIDIA V100', 'NVIDIA A100'
    gpu_compute_capability VARCHAR(20), -- e.g., '7.5', '8.0'

    -- Pricing
    price_per_hour DECIMAL(10, 4), -- On-demand price
    spot_price_per_hour DECIMAL(10, 4), -- Spot/preemptible price

    -- Availability
    is_available BOOLEAN DEFAULT true,
    supports_spot BOOLEAN DEFAULT true,

    -- Metadata
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(provider, instance_type)
);

-- Region-Instance Type Mapping (which instances are available in which regions)
CREATE TABLE IF NOT EXISTS region_instance_availability (
    id SERIAL PRIMARY KEY,
    region_id INTEGER NOT NULL REFERENCES regions(id) ON DELETE CASCADE,
    instance_type_id INTEGER NOT NULL REFERENCES instance_types(id) ON DELETE CASCADE,
    is_available BOOLEAN DEFAULT true,
    stock_status VARCHAR(50) DEFAULT 'available', -- 'available', 'limited', 'out_of_stock'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(region_id, instance_type_id)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_regions_provider ON regions(provider);
CREATE INDEX IF NOT EXISTS idx_regions_available ON regions(is_available);
CREATE INDEX IF NOT EXISTS idx_instance_types_provider ON instance_types(provider);
CREATE INDEX IF NOT EXISTS idx_instance_types_gpu_model ON instance_types(gpu_model);
CREATE INDEX IF NOT EXISTS idx_instance_types_available ON instance_types(is_available);
CREATE INDEX IF NOT EXISTS idx_region_instance_region_id ON region_instance_availability(region_id);
CREATE INDEX IF NOT EXISTS idx_region_instance_instance_type_id ON region_instance_availability(instance_type_id);

-- Insert Azure Regions
INSERT INTO regions (provider, region_code, region_name, location) VALUES
('azure', 'eastus', 'East US', 'Virginia, US'),
('azure', 'eastus2', 'East US 2', 'Virginia, US'),
('azure', 'westus', 'West US', 'California, US'),
('azure', 'westus2', 'West US 2', 'Washington, US'),
('azure', 'centralus', 'Central US', 'Iowa, US'),
('azure', 'northcentralus', 'North Central US', 'Illinois, US'),
('azure', 'southcentralus', 'South Central US', 'Texas, US'),
('azure', 'westeurope', 'West Europe', 'Netherlands'),
('azure', 'northeurope', 'North Europe', 'Ireland'),
('azure', 'southeastasia', 'Southeast Asia', 'Singapore'),
('azure', 'japaneast', 'Japan East', 'Tokyo, Japan'),
('azure', 'australiaeast', 'Australia East', 'New South Wales, Australia')
ON CONFLICT (provider, region_code) DO NOTHING;

-- Insert AWS Regions
INSERT INTO regions (provider, region_code, region_name, location) VALUES
('aws', 'us-east-1', 'US East (N. Virginia)', 'Virginia, US'),
('aws', 'us-east-2', 'US East (Ohio)', 'Ohio, US'),
('aws', 'us-west-1', 'US West (N. California)', 'California, US'),
('aws', 'us-west-2', 'US West (Oregon)', 'Oregon, US'),
('aws', 'eu-west-1', 'EU (Ireland)', 'Ireland'),
('aws', 'eu-central-1', 'EU (Frankfurt)', 'Germany'),
('aws', 'ap-southeast-1', 'Asia Pacific (Singapore)', 'Singapore'),
('aws', 'ap-northeast-1', 'Asia Pacific (Tokyo)', 'Japan'),
('aws', 'ap-southeast-2', 'Asia Pacific (Sydney)', 'Australia')
ON CONFLICT (provider, region_code) DO NOTHING;

-- Insert GCP Regions
INSERT INTO regions (provider, region_code, region_name, location) VALUES
('gcp', 'us-central1', 'Iowa', 'Iowa, US'),
('gcp', 'us-east1', 'South Carolina', 'South Carolina, US'),
('gcp', 'us-west1', 'Oregon', 'Oregon, US'),
('gcp', 'us-west4', 'Las Vegas', 'Nevada, US'),
('gcp', 'europe-west1', 'Belgium', 'Belgium'),
('gcp', 'europe-west4', 'Netherlands', 'Netherlands'),
('gcp', 'asia-southeast1', 'Singapore', 'Singapore'),
('gcp', 'asia-northeast1', 'Tokyo', 'Japan'),
('gcp', 'australia-southeast1', 'Sydney', 'Australia')
ON CONFLICT (provider, region_code) DO NOTHING;

-- Insert Azure GPU Instance Types
INSERT INTO instance_types (provider, instance_type, instance_name, vcpu_count, memory_gb, gpu_count, gpu_memory_gb, gpu_model, gpu_compute_capability, price_per_hour, spot_price_per_hour) VALUES
-- T4 Instances
('azure', 'Standard_NC4as_T4_v3', 'NC4as T4 v3', 4, 28, 1, 16, 'NVIDIA T4', '7.5', 0.526, 0.106),
('azure', 'Standard_NC8as_T4_v3', 'NC8as T4 v3', 8, 56, 1, 16, 'NVIDIA T4', '7.5', 0.752, 0.150),
('azure', 'Standard_NC16as_T4_v3', 'NC16as T4 v3', 16, 110, 1, 16, 'NVIDIA T4', '7.5', 1.203, 0.241),
('azure', 'Standard_NC64as_T4_v3', 'NC64as T4 v3', 64, 440, 4, 64, 'NVIDIA T4', '7.5', 4.813, 0.963),
-- V100 Instances
('azure', 'Standard_NC6s_v3', 'NC6s v3', 6, 112, 1, 16, 'NVIDIA V100', '7.0', 3.060, 0.612),
('azure', 'Standard_NC12s_v3', 'NC12s v3', 12, 224, 2, 32, 'NVIDIA V100', '7.0', 6.120, 1.224),
('azure', 'Standard_NC24s_v3', 'NC24s v3', 24, 448, 4, 64, 'NVIDIA V100', '7.0', 12.240, 2.448),
-- A100 Instances
('azure', 'Standard_ND96asr_v4', 'ND96asr v4', 96, 900, 8, 320, 'NVIDIA A100', '8.0', 27.200, 5.440),
('azure', 'Standard_ND96amsr_A100_v4', 'ND96amsr A100 v4', 96, 1900, 8, 640, 'NVIDIA A100 80GB', '8.0', 32.770, 6.554)
ON CONFLICT (provider, instance_type) DO NOTHING;

-- Insert AWS GPU Instance Types
INSERT INTO instance_types (provider, instance_type, instance_name, vcpu_count, memory_gb, gpu_count, gpu_memory_gb, gpu_model, gpu_compute_capability, price_per_hour, spot_price_per_hour) VALUES
-- T4 Instances (g4dn)
('aws', 'g4dn.xlarge', 'g4dn.xlarge', 4, 16, 1, 16, 'NVIDIA T4', '7.5', 0.526, 0.158),
('aws', 'g4dn.2xlarge', 'g4dn.2xlarge', 8, 32, 1, 16, 'NVIDIA T4', '7.5', 0.752, 0.226),
('aws', 'g4dn.4xlarge', 'g4dn.4xlarge', 16, 64, 1, 16, 'NVIDIA T4', '7.5', 1.204, 0.361),
('aws', 'g4dn.12xlarge', 'g4dn.12xlarge', 48, 192, 4, 64, 'NVIDIA T4', '7.5', 3.912, 1.174),
-- V100 Instances (p3)
('aws', 'p3.2xlarge', 'p3.2xlarge', 8, 61, 1, 16, 'NVIDIA V100', '7.0', 3.060, 0.918),
('aws', 'p3.8xlarge', 'p3.8xlarge', 32, 244, 4, 64, 'NVIDIA V100', '7.0', 12.240, 3.672),
('aws', 'p3.16xlarge', 'p3.16xlarge', 64, 488, 8, 128, 'NVIDIA V100', '7.0', 24.480, 7.344),
-- A100 Instances (p4d)
('aws', 'p4d.24xlarge', 'p4d.24xlarge', 96, 1152, 8, 320, 'NVIDIA A100', '8.0', 32.770, 9.831),
-- A10G Instances (g5)
('aws', 'g5.xlarge', 'g5.xlarge', 4, 16, 1, 24, 'NVIDIA A10G', '8.6', 1.006, 0.302),
('aws', 'g5.2xlarge', 'g5.2xlarge', 8, 32, 1, 24, 'NVIDIA A10G', '8.6', 1.212, 0.364),
('aws', 'g5.4xlarge', 'g5.4xlarge', 16, 64, 1, 24, 'NVIDIA A10G', '8.6', 1.624, 0.487)
ON CONFLICT (provider, instance_type) DO NOTHING;

-- Insert GCP GPU Instance Types
INSERT INTO instance_types (provider, instance_type, instance_name, vcpu_count, memory_gb, gpu_count, gpu_memory_gb, gpu_model, gpu_compute_capability, price_per_hour, spot_price_per_hour) VALUES
-- T4 Instances
('gcp', 'n1-standard-4-t4-1', 'n1-standard-4 + 1x T4', 4, 15, 1, 16, 'NVIDIA T4', '7.5', 0.593, 0.178),
('gcp', 'n1-standard-8-t4-1', 'n1-standard-8 + 1x T4', 8, 30, 1, 16, 'NVIDIA T4', '7.5', 0.827, 0.248),
('gcp', 'n1-standard-16-t4-2', 'n1-standard-16 + 2x T4', 16, 60, 2, 32, 'NVIDIA T4', '7.5', 1.528, 0.458),
-- V100 Instances
('gcp', 'n1-standard-8-v100-1', 'n1-standard-8 + 1x V100', 8, 30, 1, 16, 'NVIDIA V100', '7.0', 2.820, 0.846),
('gcp', 'n1-standard-16-v100-2', 'n1-standard-16 + 2x V100', 16, 60, 2, 32, 'NVIDIA V100', '7.0', 5.404, 1.621),
('gcp', 'n1-standard-32-v100-4', 'n1-standard-32 + 4x V100', 32, 120, 4, 64, 'NVIDIA V100', '7.0', 10.572, 3.172),
-- A100 Instances
('gcp', 'a2-highgpu-1g', 'a2-highgpu-1g', 12, 85, 1, 40, 'NVIDIA A100', '8.0', 3.935, 1.181),
('gcp', 'a2-highgpu-2g', 'a2-highgpu-2g', 24, 170, 2, 80, 'NVIDIA A100', '8.0', 7.870, 2.361),
('gcp', 'a2-highgpu-4g', 'a2-highgpu-4g', 48, 340, 4, 160, 'NVIDIA A100', '8.0', 15.740, 4.722),
('gcp', 'a2-highgpu-8g', 'a2-highgpu-8g', 96, 680, 8, 320, 'NVIDIA A100', '8.0', 31.480, 9.444)
ON CONFLICT (provider, instance_type) DO NOTHING;

-- Map instances to regions (all instances available in all regions for simplicity)
-- In production, you would selectively map based on actual cloud provider availability
INSERT INTO region_instance_availability (region_id, instance_type_id, is_available)
SELECT r.id, i.id, true
FROM regions r
CROSS JOIN instance_types i
WHERE r.provider = i.provider
ON CONFLICT (region_id, instance_type_id) DO NOTHING;

-- Update timestamps trigger
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_regions_updated_at BEFORE UPDATE ON regions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_instance_types_updated_at BEFORE UPDATE ON instance_types
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_region_instance_availability_updated_at BEFORE UPDATE ON region_instance_availability
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
