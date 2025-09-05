# Helios Development Environment Startup Script for Windows
# Usage: ./scripts/dev_up.ps1

param(
    [string]$Mode = "fast"  # "fast" or "strong"
)

Write-Host "🚀 Starting Helios development environment..." -ForegroundColor Green
Write-Host "Mode: $Mode" -ForegroundColor Cyan

# Check if Docker is running
$dockerRunning = docker info 2>$null
if (-not $dockerRunning) {
    Write-Host "❌ Docker is not running. Please start Docker Desktop." -ForegroundColor Red
    exit 1
}

# Set environment variables
$env:HELIOS_CONSISTENCY_MODE = $Mode
$env:HELIOS_LOG_LEVEL = "info"

try {
    if ($Mode -eq "strong") {
        Write-Host "🔧 Starting STRONG consistency mode with Redis + etcd..." -ForegroundColor Yellow
        docker-compose -f deploy/docker-compose.yml up -d redis etcd prometheus grafana jaeger
        Start-Sleep -Seconds 10
        
        Write-Host "⏳ Waiting for services to be ready..." -ForegroundColor Yellow
        $timeout = 60
        $elapsed = 0
        do {
            $redisReady = docker-compose -f deploy/docker-compose.yml ps --services --filter status=running | Select-String "redis"
            $etcdReady = docker-compose -f deploy/docker-compose.yml ps --services --filter status=running | Select-String "etcd"
            
            if ($redisReady -and $etcdReady) {
                Write-Host "✅ Services are ready!" -ForegroundColor Green
                break
            }
            
            Write-Host "⏳ Still waiting... ($elapsed/$timeout seconds)" -ForegroundColor Yellow
            Start-Sleep -Seconds 2
            $elapsed += 2
        } while ($elapsed -lt $timeout)
        
        if ($elapsed -ge $timeout) {
            Write-Host "❌ Timeout waiting for services" -ForegroundColor Red
            exit 1
        }
        
        Write-Host "🌐 Starting Helios services..." -ForegroundColor Green
        docker-compose -f deploy/docker-compose.yml up -d helios-gateway helios-control
        
    } else {
        Write-Host "🔧 Starting FAST consistency mode (local only)..." -ForegroundColor Yellow
        docker-compose -f deploy/docker-compose.yml up -d prometheus grafana jaeger
        
        Write-Host "🌐 Starting local Helios gateway..." -ForegroundColor Green
        # Run locally with Go
        $env:HELIOS_CONSISTENCY_MODE = "fast"
        $env:HELIOS_REDIS_ADDRESS = ""
        $env:HELIOS_ETCD_ENDPOINTS = ""
        Start-Process -FilePath "go" -ArgumentList @("run", "./cmd/helios-gateway") -NoNewWindow -PassThru
    }
    
    Write-Host ""
    Write-Host "🎉 Helios is running!" -ForegroundColor Green
    Write-Host ""
    Write-Host "📊 Services:" -ForegroundColor Cyan
    Write-Host "  • Gateway:    http://localhost:8080" -ForegroundColor White
    Write-Host "  • Control:    http://localhost:8081" -ForegroundColor White
    Write-Host "  • Prometheus: http://localhost:9090" -ForegroundColor White
    Write-Host "  • Grafana:    http://localhost:3000 (admin/admin)" -ForegroundColor White
    Write-Host "  • Jaeger:     http://localhost:16686" -ForegroundColor White
    Write-Host ""
    Write-Host "🧪 Test commands:" -ForegroundColor Cyan
    Write-Host "  curl.exe http://localhost:8080/health" -ForegroundColor Gray
    Write-Host "  curl.exe `"http://localhost:8080/allow?tenant=acme&cost=1`" -H `"X-API-Key: test-key`"" -ForegroundColor Gray
    Write-Host "  curl.exe `"http://localhost:8080/api/v1/quota/acme?resource=default`" -H `"X-API-Key: test-key`"" -ForegroundColor Gray
    Write-Host ""
    Write-Host "To stop: ./scripts/dev_down.ps1" -ForegroundColor Yellow
    
} catch {
    Write-Host "❌ Error starting services: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}