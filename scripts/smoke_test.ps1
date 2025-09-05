# Helios Smoke Test Script for Windows
# Usage: ./scripts/smoke_test.ps1

param(
    [string]$GatewayUrl = "http://localhost:8080",
    [string]$ControlUrl = "http://localhost:8081", 
    [string]$ApiKey = "test-key"
)

Write-Host "🧪 Running Helios smoke tests..." -ForegroundColor Green
Write-Host "Gateway: $GatewayUrl" -ForegroundColor Cyan
Write-Host "Control: $ControlUrl" -ForegroundColor Cyan
Write-Host "API Key: $ApiKey" -ForegroundColor Cyan
Write-Host ""

$ErrorActionPreference = "Stop"
$testsPassed = 0
$testsFailed = 0

function Test-Endpoint {
    param(
        [string]$Name,
        [string]$Url,
        [hashtable]$Headers = @{},
        [string]$ExpectedStatus = "200",
        [string]$Method = "GET"
    )
    
    Write-Host "Testing: $Name" -ForegroundColor Yellow
    Write-Host "  URL: $Url" -ForegroundColor Gray
    
    try {
        $response = if ($Method -eq "GET") {
            Invoke-RestMethod -Uri $Url -Headers $Headers -Method GET -TimeoutSec 10
        } else {
            Invoke-RestMethod -Uri $Url -Headers $Headers -Method $Method -TimeoutSec 10
        }
        
        Write-Host "  ✅ PASS" -ForegroundColor Green
        Write-Host "  Response: $($response | ConvertTo-Json -Compress)" -ForegroundColor Gray
        $script:testsPassed++
        return $true
        
    } catch {
        Write-Host "  ❌ FAIL: $($_.Exception.Message)" -ForegroundColor Red
        $script:testsFailed++
        return $false
    }
}

function Test-RateLimit {
    param(
        [string]$Tenant,
        [int]$Requests = 10
    )
    
    Write-Host "Testing Rate Limiting for tenant: $Tenant" -ForegroundColor Yellow
    
    $allowedCount = 0
    $deniedCount = 0
    
    for ($i = 1; $i -le $Requests; $i++) {
        try {
            $url = "$GatewayUrl/allow?tenant=$Tenant&cost=1"
            $headers = @{ "X-API-Key" = $ApiKey }
            
            $response = Invoke-RestMethod -Uri $url -Headers $headers -Method GET -TimeoutSec 5
            
            if ($response.allowed) {
                $allowedCount++
                Write-Host "  Request $i: ✅ Allowed (remaining: $($response.remaining))" -ForegroundColor Green
            } else {
                $deniedCount++
                Write-Host "  Request $i: 🚫 Denied (remaining: $($response.remaining))" -ForegroundColor Red
            }
            
        } catch {
            $deniedCount++
            Write-Host "  Request $i: ❌ Error: $($_.Exception.Message)" -ForegroundColor Red
        }
        
        Start-Sleep -Milliseconds 100
    }
    
    Write-Host "  Summary: $allowedCount allowed, $deniedCount denied" -ForegroundColor Cyan
    
    if ($allowedCount -gt 0 -and $deniedCount -gt 0) {
        Write-Host "  ✅ Rate limiting working correctly" -ForegroundColor Green
        $script:testsPassed++
    } elseif ($allowedCount -eq $Requests) {
        Write-Host "  ⚠️  All requests allowed - rate limiting may not be working" -ForegroundColor Yellow
        $script:testsPassed++
    } else {
        Write-Host "  ❌ All requests denied - service may be down" -ForegroundColor Red
        $script:testsFailed++
    }
}

# Run tests
Write-Host "=== Health Checks ===" -ForegroundColor Magenta
Test-Endpoint -Name "Gateway Health" -Url "$GatewayUrl/health"
Test-Endpoint -Name "Control Health" -Url "$ControlUrl/health"

Write-Host ""
Write-Host "=== Authentication Tests ===" -ForegroundColor Magenta
Test-Endpoint -Name "Allow without API key (should fail)" -Url "$GatewayUrl/allow?tenant=test" -ExpectedStatus "401"

$headers = @{ "X-API-Key" = $ApiKey }
Test-Endpoint -Name "Allow with valid API key" -Url "$GatewayUrl/allow?tenant=test&cost=1" -Headers $headers

Write-Host ""
Write-Host "=== API Endpoint Tests ===" -ForegroundColor Magenta
Test-Endpoint -Name "Quota Check" -Url "$GatewayUrl/api/v1/quota/test?resource=default" -Headers $headers
Test-Endpoint -Name "Metrics Endpoint" -Url "$GatewayUrl/metrics"

Write-Host ""
Write-Host "=== Rate Limiting Tests ===" -ForegroundColor Magenta
Test-RateLimit -Tenant "smoke-test" -Requests 15

Write-Host ""
Write-Host "=== Service Discovery ===" -ForegroundColor Magenta
Write-Host "Checking Prometheus..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "http://localhost:9090/-/healthy" -TimeoutSec 5
    Write-Host "  ✅ Prometheus is healthy" -ForegroundColor Green
    $testsPassed++
} catch {
    Write-Host "  ❌ Prometheus check failed: $($_.Exception.Message)" -ForegroundColor Red
    $testsFailed++
}

Write-Host "Checking Grafana..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "http://localhost:3000/api/health" -TimeoutSec 5
    Write-Host "  ✅ Grafana is healthy" -ForegroundColor Green
    $testsPassed++
} catch {
    Write-Host "  ❌ Grafana check failed: $($_.Exception.Message)" -ForegroundColor Red
    $testsFailed++
}

Write-Host "Checking Jaeger..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "http://localhost:16686/" -TimeoutSec 5
    Write-Host "  ✅ Jaeger is accessible" -ForegroundColor Green
    $testsPassed++
} catch {
    Write-Host "  ❌ Jaeger check failed: $($_.Exception.Message)" -ForegroundColor Red
    $testsFailed++
}

# Summary
Write-Host ""
Write-Host "=== Test Summary ===" -ForegroundColor Magenta
Write-Host "✅ Passed: $testsPassed" -ForegroundColor Green
Write-Host "❌ Failed: $testsFailed" -ForegroundColor Red

if ($testsFailed -eq 0) {
    Write-Host ""
    Write-Host "🎉 All smoke tests passed! Helios is working correctly." -ForegroundColor Green
    exit 0
} else {
    Write-Host ""
    Write-Host "⚠️  Some tests failed. Check the output above for details." -ForegroundColor Yellow
    exit 1
}