#!/bin/bash
# Development startup script for Helios

set -e

echo "🚀 Starting Helios Development Environment"

# Check if Go is available
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21+ first."
    echo "   Download from: https://golang.org/dl/"
    exit 1
fi

# Set environment variables for development
export HELIOS_LOG_LEVEL=debug
export HELIOS_CONSISTENCY_MODE=fast
export HELIOS_METRICS_ENABLED=true
export HELIOS_GATEWAY_ADDRESS=:8080
export HELIOS_METRICS_ADDRESS=:2112

echo "📦 Installing dependencies..."
go mod tidy
go mod download

echo "🏗️  Building Helios Gateway..."
go build -o bin/helios-gateway ./cmd/helios-gateway

echo "🔄 Starting Helios Gateway..."
echo "   Gateway API: http://localhost:8080"
echo "   Metrics: http://localhost:2112/metrics"
echo "   Health: http://localhost:8080/health"
echo ""
echo "🛑 Press Ctrl+C to stop"
echo ""

./bin/helios-gateway