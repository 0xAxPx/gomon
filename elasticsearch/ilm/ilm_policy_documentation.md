# GoMon ILM Policy Documentation

**Policy Name**: `aggregator-2day-policy`  
**Version**: 1  
**Created**: September 6, 2025  
**Applied To**: `aggregator-logs-000001` and future rollover indices  

## Policy Overview

This Index Lifecycle Management (ILM) policy implements a 2-day data retention strategy with automatic rollover and storage optimization for GoMon monitoring data. The policy progresses through three phases: Hot → Warm → Delete.

### Data Lifecycle Flow
```
New Index → Hot Phase (0-6h) → Warm Phase (6h-2d) → Delete Phase (2d+)
```

---

## Phase Breakdown

### 1. Hot Phase (Active Data)
**Duration**: Index creation to 6 hours  
**Purpose**: Handle active data ingestion and determine when to rollover

#### Settings
- **`min_age: "0ms"`** - Phase starts immediately when index is created
- **`set_priority: 100`** - Highest priority for search and recovery operations

#### Rollover Conditions (ANY condition triggers rollover)
- **`max_age: "6h"`** - Create new index after 6 hours
- **`max_docs: 50000`** - Create new index after 50,000 documents  
- **`max_size: "200mb"`** - Create new index after 200MB storage

### 2. Warm Phase (Optimized Storage)
**Duration**: 6 hours to 2 days  
**Purpose**: Optimize storage and reduce resource usage

#### Settings
- **`min_age: "6h"`** - Phase starts 6 hours after index creation
- **`set_priority: 50`** - Medium priority (lower than hot phase)
- **`forcemerge: max_num_segments: 1`** - Optimize index to single segment for better compression

### 3. Delete Phase (Data Removal)
**Duration**: After 2 days  
**Purpose**: Automatically remove old data

#### Settings  
- **`min_age: "2d"`** - Phase starts 2 days after index creation
- **`delete_searchable_snapshot: true`** - Remove any searchable snapshots before deletion

---

## Field Definitions

### Core ILM Fields

| Field | Purpose | Your Setting |
|-------|---------|--------------|
| `version` | Policy version number | 1 |
| `modified_date` | Last policy modification | 2025-09-06T18:55:46.788Z |
| `min_age` | When phase starts (relative to index creation) | 0ms, 6h, 2d |

### Priority Settings

| Field | Purpose | Your Setting |
|-------|---------|--------------|
| `set_priority.priority` | Search/recovery priority (higher = more important) | Hot: 100, Warm: 50 |

### Rollover Settings

| Field | Purpose | Your Setting |
|-------|---------|--------------|
| `max_age` | Time limit before creating new index | 6 hours |
| `max_docs` | Document count limit before rollover | 50,000 documents |
| `max_size` | Storage size limit before rollover | 200MB |

### Optimization Settings

| Field | Purpose | Your Setting |
|-------|---------|--------------|
| `forcemerge.max_num_segments` | Merge index into N segments for compression | 1 segment |

### Deletion Settings

| Field | Purpose | Your Setting |
|-------|---------|--------------|
| `delete_searchable_snapshot` | Remove snapshots before index deletion | true |

---

## Expected Behavior

### Typical Index Lifecycle (GoMon Data Volume)
```
aggregator-logs-000001: Created → 6h (rollover) → Optimized → 2d (deleted)
aggregator-logs-000002: Created → 6h (rollover) → Optimized → 2d (deleted)
aggregator-logs-000003: Created → 6h (rollover) → Optimized → 2d (deleted)
```

### Rollover Triggers (Most Likely First)
1. **`max_age: 6h`** - Most common trigger with your data volume (~6,250 docs per 6h)
2. **`max_docs: 50000`** - Would trigger after ~2 days of data accumulation
3. **`max_size: 200mb`** - Depends on document sizes and compression

### Storage Pattern
- **Rolling window**: Always maintains last 2 days of data
- **Storage optimization**: Indices compressed after 6 hours
- **Automatic cleanup**: Old data deleted without manual intervention

---

## Policy Status

### Currently Applied To
- `aggregator-logs-000001` (completed rollover, in warm phase)
- Future rollover indices will inherit this policy

### Expected Steady State
- **Active indices**: 8-12 indices (6-hour rollovers over 2 days)
- **Storage per index**: ~40-80MB after optimization
- **Total retention**: 2 days of monitoring data
- **Document count**: ~100,000-200,000 total documents

---

## Monitoring Commands

```bash
# Check policy status
kubectl exec deployment/elasticsearch-lb -n monitoring -- curl -s "http://localhost:9200/_ilm/policy/aggregator-2day-policy?pretty"

# Check index lifecycle status  
kubectl exec deployment/elasticsearch-lb -n monitoring -- curl -s "http://localhost:9200/aggregator-logs*/_ilm/explain" | jq '.indices[] | {index: .index, phase: .phase, action: .action}'

# Monitor storage usage
kubectl exec deployment/elasticsearch-lb -n monitoring -- curl -s "http://localhost:9200/_cat/indices/aggregator-logs*?v&h=index,docs.count,store.size,creation.date.string&s=creation.date"
```