#!/bin/bash
# Test script for Helios APIs

BASE_URL="http://localhost:8080"

echo "🧪 Testing Helios APIs"
echo "======================"

# Test 1: Health Check
echo "1️⃣  Health Check"
curl -s "$BASE_URL/health" | jq . || curl -s "$BASE_URL/health"
echo ""

# Test 2: Basic Rate Limiting
echo "2️⃣  Basic Rate Limiting"
curl -s "$BASE_URL/allow?tenant=test&api_key=demo-key" | jq . || curl -s "$BASE_URL/allow?tenant=test&api_key=demo-key"
echo ""

# Test 3: Rate Limiting with Cost
echo "3️⃣  Rate Limiting with Cost"
curl -s "$BASE_URL/allow?tenant=test&api_key=demo-key&cost=5" | jq . || curl -s "$BASE_URL/allow?tenant=test&api_key=demo-key&cost=5"
echo ""

# Test 4: Get Quota
echo "4️⃣  Get Current Quota"
curl -s "$BASE_URL/api/v1/quota/test?api_key=demo-key" | jq . || curl -s "$BASE_URL/api/v1/quota/test?api_key=demo-key"
echo ""

# Test 5: Exhaust Rate Limit
echo "5️⃣  Exhaust Rate Limit (should get 429)"
for i in {1..10}; do
    response=$(curl -s -w "%{http_code}" "$BASE_URL/allow?tenant=burst&api_key=demo-key&cost=20")
    status_code="${response: -3}"
    body="${response%???}"
    echo "Request $i: Status $status_code"
    if [ "$status_code" = "429" ]; then
        echo "✅ Rate limit working! Got 429 Too Many Requests"
        echo "$body" | jq . 2>/dev/null || echo "$body"
        break
    fi
done

echo ""
echo "🎉 API Testing Complete!"
echo ""
echo "💡 Next steps:"
echo "   - Check metrics: http://localhost:2112/metrics"
echo "   - View logs in the terminal running the gateway"
echo "   - Try different tenants and API keys"