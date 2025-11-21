#!/bin/bash
#
# List models available in Cloudflare R2
#

set -e

# Colors
GREEN='\033[0;32m'
NC='\033[0m'

R2_BUCKET="${R2_BUCKET:-crosslogic-models}"

if [ -z "$R2_ENDPOINT" ]; then
    echo "Error: R2_ENDPOINT not set"
    echo "Run: source .env"
    exit 1
fi

if [ -z "$AWS_ACCESS_KEY_ID" ] || [ -z "$AWS_SECRET_ACCESS_KEY" ]; then
    echo "Error: AWS credentials not set"
    echo "Export them:"
    echo "  export AWS_ACCESS_KEY_ID=\$R2_ACCESS_KEY"
    echo "  export AWS_SECRET_ACCESS_KEY=\$R2_SECRET_KEY"
    exit 1
fi

echo -e "${GREEN}Models in R2 bucket: $R2_BUCKET${NC}"
echo ""

# List all "directories" (prefixes) in the bucket
aws s3 ls "s3://${R2_BUCKET}/" --endpoint-url "$R2_ENDPOINT" | grep "PRE" | awk '{print $2}' | sed 's/\///' | while read model; do
    # Get size of each model
    size=$(aws s3 ls "s3://${R2_BUCKET}/${model}/" --recursive --endpoint-url "$R2_ENDPOINT" --summarize | grep "Total Size" | awk '{print $3}')
    size_gb=$(echo "scale=2; $size / 1073741824" | bc)
    printf "%-50s %10s GB\n" "$model" "$size_gb"
done

echo ""
echo "To use a model in vLLM:"
echo "  --model s3://${R2_BUCKET}/<model-name>"


