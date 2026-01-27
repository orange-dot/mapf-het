# ROJ TLA+ Formal Specifications

This directory contains TLA+ specifications for the ROJ distributed consensus protocol.

## Specifications

### `roj_consensus.tla`
Formal specification of the ROJ consensus protocol (adapted Raft for CAN-FD).

**Key Properties Verified:**
- **Election Safety**: At most one leader per term
- **Log Matching**: Logs with same index/term are identical up to that point
- **Leader Completeness**: Committed entries appear in future leaders' logs
- **State Machine Safety**: All nodes apply commands in same order

### `partition_handling.tla`
Formal specification of network partition handling.

**Key Properties Verified:**
- **Frozen Safety**: Nodes in minority partition cannot commit
- **Majority Commit Safety**: Only majority partitions can commit
- **No Conflicting Commits**: No two partitions can both commit
- **Eventual Reconciliation**: Frozen nodes recover when partition heals

## Running the Model Checker

### Prerequisites
1. Install TLA+ Toolbox or download `tla2tools.jar` from:
   https://github.com/tlaplus/tlaplus/releases

### Quick Verification (CLI)
```bash
# Verify consensus protocol
java -jar tla2tools.jar -config roj_consensus.cfg roj_consensus.tla

# Verify partition handling
java -jar tla2tools.jar -config partition_handling.cfg partition_handling.tla
```

### Using TLA+ Toolbox (GUI)
1. Open TLA+ Toolbox
2. File → Open Spec → Add New Spec
3. Select `roj_consensus.tla` or `partition_handling.tla`
4. Create new model with constants from `.cfg` file
5. Run TLC Model Checker

## Model Sizes

### Small (Quick, ~1 min)
```
Nodes = {n1, n2, n3}
MaxTerm = 2
MaxLogLength = 3
Values = {v1}
```

### Medium (Thorough, ~10 min)
```
Nodes = {n1, n2, n3, n4, n5}
MaxTerm = 3
MaxLogLength = 4
Values = {v1, v2}
```

### Large (Exhaustive, ~1 hour+)
```
Nodes = {n1, n2, n3, n4, n5, n6, n7}
MaxTerm = 4
MaxLogLength = 5
Values = {v1, v2, v3}
```

## Paper Claims Verified

| Claim | Specification | Property |
|-------|---------------|----------|
| Leader election < 100ms | roj_consensus.tla | EventuallyLeader |
| Consensus safety | roj_consensus.tla | ElectionSafety, LogMatching |
| Partition recovery < 10s | partition_handling.tla | EventualReconciliation |
| Minority freeze | partition_handling.tla | FrozenSafety |
| Majority progress | partition_handling.tla | MajorityProgress |

## File Structure
```
specs/tla/
├── README.md                  # This file
├── roj_consensus.tla          # Consensus protocol spec
├── roj_consensus.cfg          # TLC config for consensus
├── partition_handling.tla     # Partition handling spec
└── partition_handling.cfg     # TLC config for partitions
```

## References

- Lamport, L. (2002). Specifying Systems: The TLA+ Language and Tools
- Ongaro, D. & Ousterhout, J. (2014). In Search of an Understandable Consensus Algorithm (Raft)
- ROJ Paper Section III - Distributed Consensus
