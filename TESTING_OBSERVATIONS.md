# Weather Script Testing Observations

## Test Matrix

| Input | Result | Strategy Used |
|-------|--------|---------------|
| San Francisco | 54.5°F, Clear | Nominatim geocoding, cached |
| 10001 (NYC zip) | 37°F, Clear | Zippopotam API, cached |
| Tokyo | 54.8°F, Clear | Hardcoded coords fallback |
| 90210 (Beverly Hills) | 45.7°F, Clear | Hardcoded coords fallback |

## Key Findings

### 1. Resilience Through Fallbacks

The agent learned to handle geocoding API failures by building fallback strategies:

```javascript
// Fallback chain:
1. Cache check (1-hour TTL)
2. Airport code mapping (OAK → Oakland, SFO → San Francisco)
3. Hardcoded major city coordinates
4. Nominatim (OpenStreetMap) geocoding
5. Zippopotam (US zip codes)
6. Open-Meteo geocoding
7. IP-based auto-detection
8. agent.resume() for truly unknown locations
```

### 2. Memory.js Evolution

Each parallel run updated memory.js with similar patterns:
- All added hardcoded coordinates for major cities
- All added airport code mapping
- All implemented cache with 1-hour TTL
- All used the same WMO weather code decoder

**Issue**: Running 4 agents in parallel caused race conditions - each overwrote memory.js.

### 3. What the Agent Did Well

- **Self-healing**: When Open-Meteo geocoding was blocked, tried alternatives
- **Caching**: Implemented without being asked (followed script instructions)
- **Documentation**: Updated `memories/weather-apis.txt` with working services
- **Graceful degradation**: Only called agent.resume() when truly stuck

### 4. Prompt Improvements Needed

Based on observations, these prompt tweaks could help:

#### A. Teach API Discovery Patterns

```
When an API fails or is blocked:
1. Don't give up immediately
2. Try alternative services for the same data
3. Consider caching successful responses longer
4. Document what worked in memories/
```

#### B. Discourage Hardcoding

The agent hardcoded city coordinates as a fallback. While pragmatic, this doesn't scale.

Better approach: Use a more reliable primary geocoding service, or fetch a city database once and cache it.

#### C. Improve Error Messages

Current: `agent.resume("All geocoding services failed for: 90210")`

Better: Include what was tried and what failed, so the next agent iteration can try something different.

### 5. Framework Observations

#### Race Condition with Parallel Runs

When running multiple agents in parallel on the same thought, they all write to the same `memory.js`. Last one wins.

Options:
- Lock file during agent execution
- Separate cache per invocation
- Accept last-write-wins as expected behavior

#### Network Approval Prompts Blocked in Background

Background shell runs can't receive user approval for network access. This caused some geocoding to fail with "access denied" rather than prompting.

For testing, consider:
- Pre-approving domains in policy.json
- Running tests serially in foreground

### 6. CDN Package Guidance (From Plan)

The pending plan to add esm.sh documentation is still relevant. Weather script doesn't need npm packages, but more complex thoughts will.

Recommend implementing the plan to teach agents:
```javascript
// Fetch lodash from CDN and save locally
var resp = net.fetch("https://esm.sh/lodash@4.17.21?cjs&bundle");
fs.writeFile("lib/lodash.js", resp.body);
var _ = require("lib/lodash.js");
```

## Recommendations

1. **Keep fallback chain approach** - it works well
2. **Add esm.sh documentation** - per the existing plan
3. **Consider serializing agent runs** - for thoughts that write to shared state
4. **Pre-populate common data** - airport codes, major cities could be in a default memory file

## Raw Output Files

Test outputs are in:
- `/tmp/claude/tasks/b341a61.output` - San Francisco
- `/tmp/claude/tasks/b66e411.output` - 10001
- `/tmp/claude/tasks/b6f9ac2.output` - Tokyo
- `/tmp/claude/tasks/bfdbc89.output` - 90210
