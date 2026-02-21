# ThinkingScript Test Metrics

Tracking performance across multiple runs to see if prompts/memory.js improves.

## Test Runs

| Run | Example | Input | Time (s) | Status | Notes |
|-----|---------|-------|----------|--------|-------|
| 1 | hello.md | - | 3.3 | first | Creates memory.js |
| 2 | hello.md | - | 0.03 | cached | memory.js handles |
| 3 | weather.md | OAK | 0.8 | cached | Airport code in hardcoded list |
| 4 | weather.md | SFO | 0.8 | cached | Airport code in hardcoded list |
| 5 | weather.md | Austin, TX | 77.1 | first | Agent adds Austin to database |
| 6 | weather.md | Austin, TX | 0.03 | cached | Now instant from memory.js |
| 7 | stocks.md | - | ~120 | first | APIs blocked, creates sample data |
| 8 | stocks.md | - | 0.06 | cached | Instant from memory.js |

| 9 | weather.md | SFO OAK DEN FRA | 1.3 | cached | Multiple airports work |
| 10 | weather.md | Orinda | 57 | first | Agent added Orinda to database |
| 11 | weather.md | 5 locations | 1.5 | cached | Multi-location memory.js |
| 12 | weather.md | 8 locations | 0.03 | cached | All from cache, instant |

## Insights

### Convergence Pattern

Thoughts converge to self-sufficiency:
- **First run**: Agent explores, handles failures, writes memory.js
- **Second run**: memory.js handles everything, no LLM needed

### Performance Improvement

| State | Avg Time |
|-------|----------|
| First run (agent needed) | 3-120s |
| After memory.js | <1s |

That's **50-2000x faster** after the first run.

### API Resilience

When APIs are blocked, the agent:
1. Tries multiple alternative services
2. Falls back to hardcoded data
3. Documents what's blocked in memories/
4. Writes memory.js that handles both modes

### Network Access Issues

Some domains appear blocked in background shell tasks:
- `query1.finance.yahoo.com`
- `finnhub.io`
- `api.twelvedata.com`
- `nominatim.openstreetmap.org` (sometimes)
- `geocoding-api.open-meteo.com`

Always accessible:
- `api.open-meteo.com` (weather)
- `ip-api.com` (geolocation)
- `api.zippopotam.us` (zip codes)
