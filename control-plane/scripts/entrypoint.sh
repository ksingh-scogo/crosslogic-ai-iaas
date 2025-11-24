#!/bin/bash
set -e

echo "🚀 Starting CrossLogic Control Plane..."

# Azure Service Principal Login for SkyPilot
if [ -n "$AZURE_CLIENT_ID" ] && [ -n "$AZURE_CLIENT_SECRET" ] && [ -n "$AZURE_TENANT_ID" ]; then
    echo "🔐 Authenticating with Azure Service Principal..."
    az login --service-principal \
        --username "$AZURE_CLIENT_ID" \
        --password "$AZURE_CLIENT_SECRET" \
        --tenant "$AZURE_TENANT_ID" \
        --output none 2>&1 || {
            echo "❌ Azure login failed"
            exit 1
        }

    if [ -n "$AZURE_SUBSCRIPTION_ID" ]; then
        echo "✓ Setting Azure subscription: $AZURE_SUBSCRIPTION_ID"
        az account set --subscription "$AZURE_SUBSCRIPTION_ID"
    fi

    echo "✓ Azure authentication successful"
    echo "🔍 Verifying SkyPilot cloud access..."
    sky check 2>&1 | head -20
else
    echo "⚠️  Azure credentials not configured - SkyPilot will run without Azure access"
fi

# AWS Credentials (already handled by environment variables for boto3)
if [ -n "$AWS_ACCESS_KEY_ID" ] && [ -n "$AWS_SECRET_ACCESS_KEY" ]; then
    echo "⚠️  AWS credentials detected but not configured in this Azure-only deployment"
fi

# GCP Credentials (already handled by GOOGLE_APPLICATION_CREDENTIALS)
if [ -n "$GOOGLE_APPLICATION_CREDENTIALS" ]; then
    echo "✓ GCP credentials detected"
fi

echo "✅ Control plane starting..."
exec ./control-plane
