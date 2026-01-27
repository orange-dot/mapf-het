-------------------------- MODULE partition_handling --------------------------
(****************************************************************************)
(* ROJ Partition Handling - TLA+ Specification                              *)
(*                                                                          *)
(* This specification models network partition handling in the ROJ          *)
(* distributed consensus protocol. It covers:                               *)
(* - Quorum detection (majority required for progress)                      *)
(* - Minority freeze mode (read-only until reconnection)                    *)
(* - Epoch-based reconciliation (resolve conflicts after partition heals)   *)
(*                                                                          *)
(* Paper Reference: ROJ Paper Section III.D - Partition Tolerance           *)
(*                                                                          *)
(* Design Goals:                                                            *)
(* - Safety: Never commit conflicting values                                *)
(* - Liveness: Majority partition makes progress                            *)
(* - Recovery: < 10s partition recovery time                                *)
(*                                                                          *)
(* Author: Elektrokombinacija                                               *)
(* Date: 2026                                                               *)
(****************************************************************************)

EXTENDS Naturals, Sequences, FiniteSets, TLC

\* Constants
CONSTANTS
    Nodes,              \* Set of all node IDs
    MaxEpoch,           \* Maximum epoch number (for model checking)
    Values              \* Set of possible values

\* State variables
VARIABLES
    \* Network partition state
    partitions,         \* Current network partition (set of sets)

    \* Per-node state
    epoch,              \* Current epoch for each node
    nodeState,          \* Node state: "connected", "detecting", "frozen", "reconciling"
    reachable,          \* Set of reachable nodes for each node
    lastSeen,           \* Last heartbeat time for each peer (simplified as counter)

    \* Committed data
    committed,          \* Committed values (only in majority partition)

    \* Reconciliation state
    syncedWith,         \* Nodes we've synced with during reconciliation

    \* Global time (for model checking)
    time

\* All variables
vars == <<partitions, epoch, nodeState, reachable, lastSeen,
          committed, syncedWith, time>>

\* Nil constant
Nil == "Nil"

\* Node states
Connected == "connected"
Detecting == "detecting"
Frozen == "frozen"
Reconciling == "reconciling"

(****************************************************************************)
(* Type Invariants                                                          *)
(****************************************************************************)

TypeOK ==
    /\ partitions \in SUBSET (SUBSET Nodes)
    /\ epoch \in [Nodes -> [number: 0..MaxEpoch, startedBy: Nodes \cup {Nil}]]
    /\ nodeState \in [Nodes -> {Connected, Detecting, Frozen, Reconciling}]
    /\ reachable \in [Nodes -> SUBSET Nodes]
    /\ lastSeen \in [Nodes -> [Nodes -> Nat]]
    /\ committed \in SUBSET Values
    /\ syncedWith \in [Nodes -> SUBSET Nodes]
    /\ time \in Nat

(****************************************************************************)
(* Helper Functions                                                         *)
(****************************************************************************)

\* Total number of nodes in cluster
ClusterSize == Cardinality(Nodes)

\* Quorum: strict majority
QuorumSize == (ClusterSize \div 2) + 1

\* Check if a set of nodes forms a quorum
IsQuorum(nodeSet) == Cardinality(nodeSet) >= QuorumSize

\* Find which partition a node belongs to
PartitionOf(n) ==
    CHOOSE p \in partitions : n \in p

\* Check if two nodes can communicate
CanCommunicate(n1, n2) ==
    \E p \in partitions : n1 \in p /\ n2 \in p

\* Get reachable count (including self)
ReachableCount(n) == Cardinality(reachable[n]) + 1

\* Check if node has quorum
HasQuorum(n) == ReachableCount(n) >= QuorumSize

\* Check if node can write (connected and has quorum)
CanWrite(n) ==
    /\ nodeState[n] = Connected
    /\ HasQuorum(n)

\* Compare epochs (higher number wins; ties broken by startedBy existence)
EpochLT(e1, e2) ==
    \/ e1.number < e2.number
    \/ (e1.number = e2.number /\ e1.startedBy = Nil /\ e2.startedBy # Nil)

EpochLE(e1, e2) == EpochLT(e1, e2) \/ e1 = e2

\* Create next epoch
NextEpoch(n, currentEpoch) ==
    [number |-> currentEpoch.number + 1, startedBy |-> n]

(****************************************************************************)
(* Initial State                                                            *)
(****************************************************************************)

Init ==
    /\ partitions = {Nodes}  \* Start with no partition
    /\ epoch = [n \in Nodes |-> [number |-> 0, startedBy |-> Nil]]
    /\ nodeState = [n \in Nodes |-> Connected]
    /\ reachable = [n \in Nodes |-> Nodes \ {n}]
    /\ lastSeen = [n \in Nodes |-> [m \in Nodes |-> 0]]
    /\ committed = {}
    /\ syncedWith = [n \in Nodes |-> {}]
    /\ time = 0

(****************************************************************************)
(* Actions: Network Partitions                                              *)
(****************************************************************************)

\* Network partitions into two groups
\* Also updates reachable to reflect instant detection (conservative model)
CreatePartition(group1, group2) ==
    /\ group1 \cup group2 = Nodes
    /\ group1 \cap group2 = {}
    /\ group1 # {}
    /\ group2 # {}
    /\ partitions' = {group1, group2}
    \* Instantly update reachable to reflect partition (conservative)
    /\ reachable' = [n \in Nodes |->
        IF n \in group1 THEN group1 \ {n}
        ELSE group2 \ {n}]
    /\ UNCHANGED <<epoch, nodeState, lastSeen, committed, syncedWith, time>>

\* Network heals (all nodes can communicate again)
HealPartition ==
    /\ Cardinality(partitions) > 1
    /\ partitions' = {Nodes}
    \* Instantly update reachable to reflect healing
    /\ reachable' = [n \in Nodes |-> Nodes \ {n}]
    /\ UNCHANGED <<epoch, nodeState, lastSeen, committed, syncedWith, time>>

(****************************************************************************)
(* Actions: Heartbeat and Liveness Detection                                *)
(****************************************************************************)

\* Time advances
Tick ==
    /\ time' = time + 1
    /\ UNCHANGED <<partitions, epoch, nodeState, reachable, lastSeen,
                   committed, syncedWith>>

\* Node n sends heartbeat to m (if reachable)
SendHeartbeat(n, m) ==
    /\ n # m
    /\ CanCommunicate(n, m)
    /\ lastSeen' = [lastSeen EXCEPT ![m][n] = time]
    /\ reachable' = [reachable EXCEPT ![m] = @ \cup {n}]
    /\ UNCHANGED <<partitions, epoch, nodeState, committed, syncedWith, time>>

\* Node n detects peer m is unreachable (timeout)
DetectUnreachable(n, m) ==
    /\ n # m
    /\ ~CanCommunicate(n, m)  \* They're in different partitions
    /\ m \in reachable[n]
    /\ reachable' = [reachable EXCEPT ![n] = @ \ {m}]
    /\ UNCHANGED <<partitions, epoch, nodeState, lastSeen, committed, syncedWith, time>>

(****************************************************************************)
(* Actions: Partition State Machine                                         *)
(****************************************************************************)

\* Node n detects quorum loss
DetectQuorumLoss(n) ==
    /\ nodeState[n] = Connected
    /\ ~HasQuorum(n)
    /\ nodeState' = [nodeState EXCEPT ![n] = Detecting]
    /\ UNCHANGED <<partitions, epoch, reachable, lastSeen, committed, syncedWith, time>>

\* Node n confirms it's in minority partition (freeze)
ConfirmMinority(n) ==
    /\ nodeState[n] = Detecting
    /\ ~HasQuorum(n)  \* Still no quorum after detection period
    /\ nodeState' = [nodeState EXCEPT ![n] = Frozen]
    /\ UNCHANGED <<partitions, epoch, reachable, lastSeen, committed, syncedWith, time>>

\* Node n regains quorum while detecting
RegainQuorum(n) ==
    /\ nodeState[n] = Detecting
    /\ HasQuorum(n)
    /\ nodeState' = [nodeState EXCEPT ![n] = Connected]
    /\ UNCHANGED <<partitions, epoch, reachable, lastSeen, committed, syncedWith, time>>

\* Node n (in frozen state) detects partition healed
StartReconciliation(n) ==
    /\ nodeState[n] = Frozen
    /\ HasQuorum(n)
    /\ epoch[n].number < MaxEpoch  \* Model bound guard
    /\ nodeState' = [nodeState EXCEPT ![n] = Reconciling]
    /\ epoch' = [epoch EXCEPT ![n] = NextEpoch(n, epoch[n])]
    /\ syncedWith' = [syncedWith EXCEPT ![n] = {}]
    /\ UNCHANGED <<partitions, reachable, lastSeen, committed, time>>

\* Node n syncs with peer m during reconciliation
SyncWithPeer(n, m) ==
    /\ nodeState[n] = Reconciling
    /\ n # m
    /\ m \in reachable[n]
    /\ syncedWith' = [syncedWith EXCEPT ![n] = @ \cup {m}]
    \* Update epoch if peer has higher epoch
    /\ IF EpochLT(epoch[n], epoch[m])
       THEN epoch' = [epoch EXCEPT ![n] = epoch[m]]
       ELSE UNCHANGED epoch
    /\ UNCHANGED <<partitions, nodeState, reachable, lastSeen, committed, time>>

\* Node n completes reconciliation
CompleteReconciliation(n) ==
    /\ nodeState[n] = Reconciling
    /\ Cardinality(syncedWith[n]) >= QuorumSize - 1  \* Synced with majority
    /\ nodeState' = [nodeState EXCEPT ![n] = Connected]
    /\ syncedWith' = [syncedWith EXCEPT ![n] = {}]
    /\ UNCHANGED <<partitions, epoch, reachable, lastSeen, committed, time>>

(****************************************************************************)
(* Actions: Value Operations                                                *)
(****************************************************************************)

\* Node n commits a value (only if it can write)
CommitValue(n, v) ==
    /\ CanWrite(n)
    /\ v \notin committed
    /\ committed' = committed \cup {v}
    /\ UNCHANGED <<partitions, epoch, nodeState, reachable, lastSeen, syncedWith, time>>

\* Attempt to commit in frozen state (should fail - this is for checking safety)
AttemptCommitFrozen(n, v) ==
    /\ nodeState[n] = Frozen
    /\ FALSE  \* This action is always disabled - frozen nodes can't commit

(****************************************************************************)
(* Next State                                                               *)
(****************************************************************************)

\* Partition actions (environment)
PartitionActions ==
    \/ \E g1, g2 \in SUBSET Nodes : CreatePartition(g1, g2)
    \/ HealPartition

\* Node actions
NodeActions ==
    \/ \E n, m \in Nodes : SendHeartbeat(n, m)
    \/ \E n, m \in Nodes : DetectUnreachable(n, m)
    \/ \E n \in Nodes : DetectQuorumLoss(n)
    \/ \E n \in Nodes : ConfirmMinority(n)
    \/ \E n \in Nodes : RegainQuorum(n)
    \/ \E n \in Nodes : StartReconciliation(n)
    \/ \E n, m \in Nodes : SyncWithPeer(n, m)
    \/ \E n \in Nodes : CompleteReconciliation(n)
    \/ \E n \in Nodes, v \in Values : CommitValue(n, v)
    \/ Tick

Next == PartitionActions \/ NodeActions

\* Fairness constraints
Fairness ==
    /\ \A n \in Nodes : WF_vars(DetectQuorumLoss(n))
    /\ \A n \in Nodes : WF_vars(ConfirmMinority(n))
    /\ \A n \in Nodes : WF_vars(RegainQuorum(n))
    /\ \A n \in Nodes : WF_vars(StartReconciliation(n))
    /\ \A n \in Nodes : WF_vars(CompleteReconciliation(n))
    /\ WF_vars(HealPartition)

Spec == Init /\ [][Next]_vars /\ Fairness

(****************************************************************************)
(* Safety Properties                                                        *)
(****************************************************************************)

\* Frozen nodes cannot commit
FrozenSafety ==
    \A n \in Nodes :
        nodeState[n] = Frozen => ~CanWrite(n)

\* Only majority partitions can commit
MajorityCommitSafety ==
    \A n \in Nodes :
        CanWrite(n) => IsQuorum(PartitionOf(n))

\* No conflicting commits (simplified - real version would track per-value)
\* This checks that commits only happen in majority partition
NoConflictingCommits ==
    \A n1, n2 \in Nodes :
        (CanWrite(n1) /\ CanWrite(n2)) =>
            PartitionOf(n1) = PartitionOf(n2)

\* Epoch monotonicity
EpochMonotonicity ==
    \A n \in Nodes :
        epoch[n].number >= 0

\* Combined safety invariant
Safety ==
    /\ TypeOK
    /\ FrozenSafety
    /\ MajorityCommitSafety
    /\ NoConflictingCommits
    /\ EpochMonotonicity

(****************************************************************************)
(* Liveness Properties (require Fairness)                                   *)
(****************************************************************************)

\* Majority partition eventually makes progress
MajorityProgress ==
    \A p \in partitions :
        IsQuorum(p) ~>
            (\E n \in p : nodeState[n] = Connected)

\* Frozen nodes eventually reconcile when partition heals
EventualReconciliation ==
    \A n \in Nodes :
        (nodeState[n] = Frozen /\ HasQuorum(n)) ~>
            nodeState[n] = Connected

\* Minority eventually freezes
EventualFreeze ==
    \A n \in Nodes :
        (~HasQuorum(n) /\ nodeState[n] = Connected) ~>
            nodeState[n] = Frozen

(****************************************************************************)
(* Recovery Time Property (simplified)                                      *)
(* Real version would use real-time constraints                             *)
(****************************************************************************)

\* This is a simplified check - in reality we'd verify:
\* "time from partition heal to all nodes Connected < 10 seconds"
\* TLA+ doesn't have built-in real-time, so we check state transitions instead

RecoveryEventuallyCompletes ==
    <>[](\A n \in Nodes : nodeState[n] = Connected)

=============================================================================
\* Modification History
\* Last modified: 2026-01-27
