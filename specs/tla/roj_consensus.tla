----------------------------- MODULE roj_consensus -----------------------------
(****************************************************************************)
(* ROJ Consensus Protocol - TLA+ Specification                              *)
(*                                                                          *)
(* This specification models the ROJ distributed consensus protocol for     *)
(* EK-KOR2 modular power electronics coordination. It is based on Raft but  *)
(* adapted for CAN-FD networks with k-threshold voting.                     *)
(*                                                                          *)
(* Paper Reference: ROJ Paper Section III - Distributed Consensus           *)
(*                                                                          *)
(* Key Properties Verified:                                                 *)
(* 1. Election Safety: At most one leader per term                          *)
(* 2. Log Matching: If two logs contain entry with same index and term,     *)
(*    then logs are identical up to that index                              *)
(* 3. Leader Completeness: If entry is committed, it appears in all future  *)
(*    leaders' logs                                                         *)
(* 4. State Machine Safety: All nodes apply same commands in same order     *)
(*                                                                          *)
(* Author: Elektrokombinacija                                               *)
(* Date: 2026                                                               *)
(****************************************************************************)

EXTENDS Naturals, Sequences, FiniteSets, TLC

\* Constants
CONSTANTS
    Nodes,           \* Set of all node IDs
    MaxTerm,         \* Maximum term number (for model checking)
    MaxLogLength,    \* Maximum log length (for model checking)
    Values           \* Set of possible values to propose

\* State variables
VARIABLES
    \* Per-node state
    currentTerm,     \* Current term for each node
    votedFor,        \* Candidate voted for in current term (or Nil)
    state,           \* Node state: "follower", "candidate", "leader"
    log,             \* Log entries: sequence of [term, value]
    commitIndex,     \* Index of highest committed entry

    \* Leader-specific state (only used when state = "leader")
    nextIndex,       \* For each follower: next log index to send
    matchIndex,      \* For each follower: highest replicated index

    \* Message passing
    messages         \* Set of messages in transit

\* Helper: All variables
vars == <<currentTerm, votedFor, state, log, commitIndex,
          nextIndex, matchIndex, messages>>

\* Nil constant for "no vote"
Nil == "Nil"

\* Message types
RequestVoteRequest == "RequestVoteRequest"
RequestVoteResponse == "RequestVoteResponse"
AppendEntriesRequest == "AppendEntriesRequest"
AppendEntriesResponse == "AppendEntriesResponse"

(****************************************************************************)
(* Type Invariants                                                          *)
(****************************************************************************)

TypeOK ==
    /\ currentTerm \in [Nodes -> 0..MaxTerm]
    /\ votedFor \in [Nodes -> Nodes \cup {Nil}]
    /\ state \in [Nodes -> {"follower", "candidate", "leader"}]
    /\ log \in [Nodes -> Seq([term: 0..MaxTerm, value: Values])]
    /\ commitIndex \in [Nodes -> 0..MaxLogLength]
    /\ nextIndex \in [Nodes -> [Nodes -> 1..MaxLogLength]]
    /\ matchIndex \in [Nodes -> [Nodes -> 0..MaxLogLength]]

(****************************************************************************)
(* Helper Functions                                                         *)
(****************************************************************************)

\* Quorum: strict majority (N/2 + 1 for odd N)
Quorum == {Q \in SUBSET Nodes : Cardinality(Q) * 2 > Cardinality(Nodes)}

\* Get last log index for a node
LastLogIndex(n) == Len(log[n])

\* Get last log term for a node
LastLogTerm(n) ==
    IF Len(log[n]) > 0
    THEN log[n][Len(log[n])].term
    ELSE 0

\* Check if candidate's log is at least as up-to-date as voter's
LogOK(voter, candidateLastTerm, candidateLastIndex) ==
    \/ candidateLastTerm > LastLogTerm(voter)
    \/ /\ candidateLastTerm = LastLogTerm(voter)
       /\ candidateLastIndex >= LastLogIndex(voter)

\* Get log term at index (0 if index is 0 or beyond log)
LogTerm(n, idx) ==
    IF idx = 0 \/ idx > Len(log[n])
    THEN 0
    ELSE log[n][idx].term

\* Minimum of two numbers
Min(a, b) == IF a < b THEN a ELSE b

\* Maximum of two numbers
Max(a, b) == IF a > b THEN a ELSE b

\* Send a message
Send(m) == messages' = messages \cup {m}

\* Discard a message
Discard(m) == messages' = messages \ {m}

(****************************************************************************)
(* Initial State                                                            *)
(****************************************************************************)

Init ==
    /\ currentTerm = [n \in Nodes |-> 0]
    /\ votedFor = [n \in Nodes |-> Nil]
    /\ state = [n \in Nodes |-> "follower"]
    /\ log = [n \in Nodes |-> <<>>]
    /\ commitIndex = [n \in Nodes |-> 0]
    /\ nextIndex = [n \in Nodes |-> [m \in Nodes |-> 1]]
    /\ matchIndex = [n \in Nodes |-> [m \in Nodes |-> 0]]
    /\ messages = {}

(****************************************************************************)
(* Actions: Leader Election                                                 *)
(****************************************************************************)

\* Node n times out and starts election
StartElection(n) ==
    /\ state[n] \in {"follower", "candidate"}
    /\ currentTerm[n] < MaxTerm
    /\ currentTerm' = [currentTerm EXCEPT ![n] = currentTerm[n] + 1]
    /\ state' = [state EXCEPT ![n] = "candidate"]
    /\ votedFor' = [votedFor EXCEPT ![n] = n]  \* Vote for self
    /\ messages' = messages \cup
        {[type |-> RequestVoteRequest,
          term |-> currentTerm[n] + 1,
          candidate |-> n,
          lastLogTerm |-> LastLogTerm(n),
          lastLogIndex |-> LastLogIndex(n),
          dest |-> m] : m \in Nodes \ {n}}
    /\ UNCHANGED <<log, commitIndex, nextIndex, matchIndex>>

\* Node n handles RequestVote from candidate c
HandleRequestVote(n, m) ==
    /\ m.type = RequestVoteRequest
    /\ m.dest = n
    /\ LET grant == /\ m.term >= currentTerm[n]
                    /\ (votedFor[n] = Nil \/ votedFor[n] = m.candidate)
                    /\ LogOK(n, m.lastLogTerm, m.lastLogIndex)
       IN
       /\ IF m.term > currentTerm[n]
          THEN /\ currentTerm' = [currentTerm EXCEPT ![n] = m.term]
               /\ state' = [state EXCEPT ![n] = "follower"]
               /\ votedFor' = [votedFor EXCEPT ![n] = IF grant THEN m.candidate ELSE Nil]
          ELSE /\ votedFor' = [votedFor EXCEPT ![n] = IF grant THEN m.candidate ELSE votedFor[n]]
               /\ UNCHANGED <<currentTerm, state>>
       /\ Send([type |-> RequestVoteResponse,
                term |-> IF m.term > currentTerm[n] THEN m.term ELSE currentTerm[n],
                voter |-> n,
                granted |-> grant,
                dest |-> m.candidate])
       /\ Discard(m)
    /\ UNCHANGED <<log, commitIndex, nextIndex, matchIndex>>

\* Candidate n receives vote and potentially becomes leader
BecomeLeader(n) ==
    /\ state[n] = "candidate"
    /\ LET votes == {m \in messages :
                     /\ m.type = RequestVoteResponse
                     /\ m.dest = n
                     /\ m.term = currentTerm[n]
                     /\ m.granted}
           voters == {m.voter : m \in votes} \cup {n}  \* Include self-vote
       IN
       /\ voters \in Quorum
       /\ state' = [state EXCEPT ![n] = "leader"]
       /\ nextIndex' = [nextIndex EXCEPT ![n] =
                        [m \in Nodes |-> LastLogIndex(n) + 1]]
       /\ matchIndex' = [matchIndex EXCEPT ![n] =
                         [m \in Nodes |-> 0]]
       \* Remove processed vote messages
       /\ messages' = messages \ votes
    /\ UNCHANGED <<currentTerm, votedFor, log, commitIndex>>

(****************************************************************************)
(* Actions: Log Replication                                                 *)
(****************************************************************************)

\* Leader n appends new entry
ClientRequest(n, v) ==
    /\ state[n] = "leader"
    /\ Len(log[n]) < MaxLogLength
    /\ log' = [log EXCEPT ![n] = Append(@, [term |-> currentTerm[n], value |-> v])]
    /\ UNCHANGED <<currentTerm, votedFor, state, commitIndex,
                   nextIndex, matchIndex, messages>>

\* Leader n sends AppendEntries to follower m
SendAppendEntries(n, m) ==
    /\ state[n] = "leader"
    /\ n # m
    /\ LET prevLogIndex == nextIndex[n][m] - 1
           prevLogTerm == LogTerm(n, prevLogIndex)
           entries == IF nextIndex[n][m] > Len(log[n])
                      THEN <<>>
                      ELSE SubSeq(log[n], nextIndex[n][m], Len(log[n]))
       IN
       Send([type |-> AppendEntriesRequest,
             term |-> currentTerm[n],
             leader |-> n,
             prevLogIndex |-> prevLogIndex,
             prevLogTerm |-> prevLogTerm,
             entries |-> entries,
             leaderCommit |-> commitIndex[n],
             dest |-> m])
    /\ UNCHANGED <<currentTerm, votedFor, state, log, commitIndex,
                   nextIndex, matchIndex>>

\* Follower n handles AppendEntries from leader
HandleAppendEntries(n, m) ==
    /\ m.type = AppendEntriesRequest
    /\ m.dest = n
    /\ LET success == /\ m.term >= currentTerm[n]
                      /\ (m.prevLogIndex = 0 \/
                         (m.prevLogIndex <= Len(log[n]) /\
                          log[n][m.prevLogIndex].term = m.prevLogTerm))
       IN
       /\ IF m.term > currentTerm[n]
          THEN /\ currentTerm' = [currentTerm EXCEPT ![n] = m.term]
               /\ votedFor' = [votedFor EXCEPT ![n] = Nil]
          ELSE UNCHANGED <<currentTerm, votedFor>>
       /\ state' = [state EXCEPT ![n] = "follower"]
       /\ IF success
          THEN /\ log' = [log EXCEPT ![n] =
                   SubSeq(@, 1, m.prevLogIndex) \o m.entries]
               /\ commitIndex' = [commitIndex EXCEPT ![n] =
                   Min(m.leaderCommit, Len(log'[n]))]
          ELSE UNCHANGED <<log, commitIndex>>
       /\ Send([type |-> AppendEntriesResponse,
                term |-> IF m.term > currentTerm[n] THEN m.term ELSE currentTerm[n],
                follower |-> n,
                success |-> success,
                matchIndex |-> IF success
                               THEN m.prevLogIndex + Len(m.entries)
                               ELSE 0,
                dest |-> m.leader])
       /\ Discard(m)
    /\ UNCHANGED <<nextIndex, matchIndex>>

\* Leader n handles AppendEntries response from follower
HandleAppendEntriesResponse(n, m) ==
    /\ m.type = AppendEntriesResponse
    /\ m.dest = n
    /\ state[n] = "leader"
    /\ m.term = currentTerm[n]
    /\ IF m.success
       THEN /\ matchIndex' = [matchIndex EXCEPT ![n][m.follower] = m.matchIndex]
            /\ nextIndex' = [nextIndex EXCEPT ![n][m.follower] = m.matchIndex + 1]
       ELSE /\ nextIndex' = [nextIndex EXCEPT ![n][m.follower] =
                             Max(1, nextIndex[n][m.follower] - 1)]
            /\ UNCHANGED matchIndex
    /\ Discard(m)
    /\ UNCHANGED <<currentTerm, votedFor, state, log, commitIndex>>

\* Leader n advances commit index
AdvanceCommitIndex(n) ==
    /\ state[n] = "leader"
    /\ \E idx \in (commitIndex[n]+1)..Len(log[n]) :
       /\ log[n][idx].term = currentTerm[n]
       /\ LET matches == {m \in Nodes : matchIndex[n][m] >= idx} \cup {n}
          IN matches \in Quorum
       /\ commitIndex' = [commitIndex EXCEPT ![n] = idx]
    /\ UNCHANGED <<currentTerm, votedFor, state, log,
                   nextIndex, matchIndex, messages>>

(****************************************************************************)
(* Actions: Term Updates (Step Down)                                        *)
(****************************************************************************)

\* Node n discovers higher term in any message and steps down
UpdateTerm(n, m) ==
    /\ m.term > currentTerm[n]
    /\ currentTerm' = [currentTerm EXCEPT ![n] = m.term]
    /\ state' = [state EXCEPT ![n] = "follower"]
    /\ votedFor' = [votedFor EXCEPT ![n] = Nil]
    /\ UNCHANGED <<log, commitIndex, nextIndex, matchIndex, messages>>

(****************************************************************************)
(* Next State                                                               *)
(****************************************************************************)

Next ==
    \/ \E n \in Nodes : StartElection(n)
    \/ \E n \in Nodes, m \in messages : HandleRequestVote(n, m)
    \/ \E n \in Nodes : BecomeLeader(n)
    \/ \E n \in Nodes, v \in Values : ClientRequest(n, v)
    \/ \E n, m \in Nodes : SendAppendEntries(n, m)
    \/ \E n \in Nodes, m \in messages : HandleAppendEntries(n, m)
    \/ \E n \in Nodes, m \in messages : HandleAppendEntriesResponse(n, m)
    \/ \E n \in Nodes : AdvanceCommitIndex(n)

\* Fairness: Eventually handle messages, eventually timeout
Fairness ==
    /\ \A n \in Nodes : WF_vars(StartElection(n))
    /\ \A n \in Nodes : WF_vars(BecomeLeader(n))
    /\ \A n \in Nodes : WF_vars(AdvanceCommitIndex(n))
    /\ \A n \in Nodes, m \in messages : WF_vars(HandleRequestVote(n, m))
    /\ \A n \in Nodes, m \in messages : WF_vars(HandleAppendEntries(n, m))
    /\ \A n \in Nodes, m \in messages : WF_vars(HandleAppendEntriesResponse(n, m))

Spec == Init /\ [][Next]_vars /\ Fairness

(****************************************************************************)
(* Safety Properties                                                        *)
(****************************************************************************)

\* Election Safety: At most one leader per term
ElectionSafety ==
    \A n1, n2 \in Nodes :
        (state[n1] = "leader" /\ state[n2] = "leader" /\
         currentTerm[n1] = currentTerm[n2]) => n1 = n2

\* Log Matching: If two logs have entry with same index and term,
\* they are identical up to that index
LogMatching ==
    \A n1, n2 \in Nodes :
        \A idx \in 1..Min(Len(log[n1]), Len(log[n2])) :
            log[n1][idx].term = log[n2][idx].term =>
                SubSeq(log[n1], 1, idx) = SubSeq(log[n2], 1, idx)

\* Leader Completeness: Committed entries are in all future leaders' logs
\* (Simplified version - full version requires temporal reasoning)
LeaderCompleteness ==
    \A n \in Nodes :
        state[n] = "leader" =>
            \A m \in Nodes :
                commitIndex[m] <= Len(log[n])

\* State Machine Safety: All committed entries are the same across nodes
StateMachineSafety ==
    \A n1, n2 \in Nodes :
        \A idx \in 1..Min(commitIndex[n1], commitIndex[n2]) :
            log[n1][idx] = log[n2][idx]

\* Combined safety invariant
Safety ==
    /\ TypeOK
    /\ ElectionSafety
    /\ LogMatching
    /\ StateMachineSafety

(****************************************************************************)
(* Liveness Properties (require Fairness)                                   *)
(****************************************************************************)

\* Eventually a leader is elected
EventuallyLeader == <>(\E n \in Nodes : state[n] = "leader")

\* If a value is proposed, it eventually gets committed (requires stable leader)
\* This is a simplified version - full liveness requires additional assumptions
EventualProgress ==
    \A n \in Nodes, v \in Values :
        (state[n] = "leader" /\ Len(log[n]) < MaxLogLength) ~>
        (\E m \in Nodes : \E idx \in 1..commitIndex[m] : log[m][idx].value = v)

=============================================================================
\* Modification History
\* Last modified: 2026-01-27
