# Helios Development Environment Shutdown Script for Windows
# Usage: ./scripts/dev_down.ps1

Write-Host "🛑 Stopping Helios development environment..." -ForegroundColor Yellow

try {
    # Stop and remove all containers
    Write-Host "📦 Stopping Docker containers..." -ForegroundColor Cyan
    docker-compose -f deploy/docker-compose.yml down
    
    # Kill any local Go processes
    Write-Host "🔪 Stopping local processes..." -ForegroundColor Cyan
    $processes = Get-Process -Name "helios-gateway", "helios-control" -ErrorAction SilentlyContinue
    if ($processes) {
        $processes | Stop-Process -Force
        Write-Host "✅ Stopped local Helios processes" -ForegroundColor Green
    }
    
    # Optional: Remove volumes (uncomment if you want to clean data)
    # Write-Host "🗑️ Removing volumes..." -ForegroundColor Cyan
    # docker-compose -f deploy/docker-compose.yml down -v
    
    Write-Host "✅ Helios development environment stopped" -ForegroundColor Green
    
} catch {
    Write-Host "❌ Error stopping services: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "You may need to manually stop Docker containers" -ForegroundColor Yellow
    exit 1
}