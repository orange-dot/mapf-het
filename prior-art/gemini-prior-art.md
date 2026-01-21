Formal Specification of Novel Algorithmic Approaches in the MAPF-HET System
Document: Formal Specification of Novel Algorithmic Approaches Version: 1.0 Date: 2026-01-21

Table of Contents
Introduction and Motivation 2.(#2-mixed-dimensional-mapf-md-mapf) 3.(#3-dimensional-conflict-decomposition) 4.(#4-energy-constrained-cbs-e-cbs)


7.(#7-formal-definitions)


1. Introduction and Motivation
1.1 Problem Setting: The Heterogeneity Gap
The domain of Multi-Agent Path Finding (MAPF) has historically focused on the coordination of homogeneous agents—standardized entities often visualized as "pebbles" on a graph—navigating a shared environment to reach distinct goal states without collision. Classical approaches, such as A* and its cooperative variants, have laid a robust theoretical foundation for these problems, optimizing for metrics like makespan or sum-of-costs. However, the idealized abstraction of homogeneous agents moving on discrete grids with uniform velocity and zero inertia is rapidly becoming insufficient for next-generation cyber-physical systems. Modern industrial ecosystems, including automated ports, smart warehousing, and multimodal logistics hubs, demand the simultaneous coordination of fleets that are fundamentally heterogeneous in their kinematic properties, operating dimensions, and energetic constraints.

The MAPF-HET system addresses this critical gap by formalizing the interactions within a mixed-dimensional workforce. Real-world applications now routinely integrate diverse robotic platforms: mobile Automated Guided Vehicles (AGVs) that navigate 2D planar surfaces, rail-bound shuttles constrained to 1D linear networks, and Unmanned Aerial Vehicles (UAVs) operating in 3D volumetric airspace. This integration creates a "Mixed-Dimensional" planning problem (MD-MAPF) where the underlying graph topology is not uniform but fragmented into specific manifolds—G 
rail
​
 , G 
floor
​
 , and G 
air
​
 —accessible only to specific agent classes.

Consider the operational complexity of a robotic battery swap station for electric buses, a scenario representative of high-density infrastructure automation. In such a system, the coordination challenge is tripartite:

Type A (Mobile Robots): These agents, analogous to standard warehouse AGVs, possess omnidirectional or differential drive capabilities on the 2D plane (v≈0.5 m/s). They are subject to planar congestion and must navigate dynamic obstacles, yet they possess the freedom to detour laterally.

Type B (Rail Robots): These high-speed shuttles (v≈2.0 m/s) are kinematically constrained to a 1D path. Their motion is physically restricted to the rail graph, meaning they cannot steer around obstacles; they must rely on precise scheduling and the use of sidings or loops to resolve "linear" conflicts.

Type C (Drones): UAVs introduce a 3D component (v≈15.0 m/s), navigating a volumetric space discretized into altitude layers. Their planning is complicated by limited battery life and the aerodynamic necessity of maintaining safety corridors to prevent downwash interference.

Standard MAPF solvers, including Conflict-Based Search (CBS) and Priority-Based Search (PBS), struggle to model these disparate interactions efficiently. A collision between two rail robots on a single track segment implies a blockage that requires one agent to retreat significantly, a resolution strategy vastly different from a momentary pause required for two crossing drones. Furthermore, standard algorithms typically assume exogenous task generation; they do not inherently account for the endogenous generation of maintenance tasks, such as charging, which are critical for the sustained operation of energy-constrained agents like drones.

1.2 Contributions
To address these challenges, this report formally specifies four major algorithmic contributions implemented within the MAPF-HET system:

MD-MAPF Formalization: We provide a rigorous mathematical definition of the Mixed-Dimensional MAPF problem. This includes the introduction of agent dimensionality functions κ and vertex compatibility masks δ, extending standard graph-based definitions to support topometric maps where different agents perceive different connectivities.

Dimensional Conflict Decomposition: We introduce a novel taxonomy of conflict types based on the dimensionality of the interacting agents. By classifying conflicts into categories such as Linear, Planar, and Vertical, we can apply specialized resolution strategies within the CBS framework. This decomposition exploits the specific geometric constraints of each conflict type (e.g., the "locking" nature of rail conflicts) to prune the search tree more effectively than generic geometric collision detection.

Energy-Constrained CBS (E-CBS): We extend the low-level solver of CBS to include a continuous energy variable. A key contribution here is the Endogenous Task Generation mechanism, where the solver automatically inserts waypoints to charging stations when an agent’s projected energy drops below a safety threshold. This transforms the problem from simple pathfinding to a combined routing and scheduling problem with resource constraints.

Layered Airspace Model: To manage the complexity of 3D pathfinding, we formalize a discrete layered airspace model. This segregates traffic by altitude and restricts vertical transitions to designated "corridors," effectively reducing the 3D planning problem into a series of 2D layer assignments coupled with vertical synchronization constraints.

2. Mixed-Dimensional MAPF (MD-MAPF)
2.1 Formal Definition
The classical MAPF problem definition is insufficient for environments where the validity of a vertex or edge is dependent on the agent's physical nature. We therefore introduce the MD-MAPF Instance.

Definition 2.1 (MD-MAPF Instance). A mixed-dimensional MAPF problem is defined as a tuple:

I=(G,A,T,κ,δ)
where:

G=(V,E) represents the global workspace graph, encompassing all physical locations.

A={a 
1
​
 ,a 
2
​
 ,…,a 
n
​
 } is the set of heterogeneous agents.

T={t 
1
​
 ,t 
2
​
 ,…,t 
m
​
 } is the set of tasks, where each task defines a start vertex and a goal vertex.

κ:A→{1,2,3} is the Agent Dimensionality Function.

δ:V→P({1,2,3}) is the Vertex Compatibility Function.

Definition 2.2 (Agent Dimensionality). The function κ(a) partitions the agent set A into three disjoint subsets based on their kinematic manifold:

This classification allows the solver to associate specific heuristic functions and low-level planners with each agent type. For instance, rail agents may utilize a specialized 1D search that accounts for switch delays, while drones utilize a 3D A* variant that penalizes altitude changes.

Definition 2.3 (Vertex Compatibility). The connectivity of the graph is subjective. A vertex v is valid for agent a if and only if κ(a)∈δ(v). This formally expresses the physical constraints of the environment:

Rail Vertices (V 
rail
​
 ): {v∣1∈δ(v)}. These form the rail network.

Floor Vertices (V 
floor
​
 ): {v∣2∈δ(v)}. These form the navigable floor area.

Air Vertices (V 
air
​
 ): {v∣3∈δ(v)}. These form the allowable airspace lattice.

Crucially, the sets V 
rail
​
 , V 
floor
​
 , and V 
air
​
  are not disjoint. Their intersections represent Interaction Manifolds. For example, a vertex v where δ(v)={2,3} represents a landing pad where a mobile robot and a drone can interact (e.g., for cargo handoff). A vertex where δ(v)={1,2} might represent a level crossing where a rail line intersects a robot path.

2.2 Space-Time Representation
To facilitate conflict detection across these heterogeneous dimensions, we employ a unified Space-Time State representation that normalizes the disparate kinematic variables.

Definition 2.4 (Augmented Space-Time State). For an agent of dimensionality d, the state s at time t is defined as:

s=(v,t,λ,θ)
v∈V: The topological location (node ID).

t∈R 
≥0
​
 : The continuous time.

λ∈Λ: A dimension-specific auxiliary variable.

For Rail (d=1): λ∈R represents the continuous position along the specific rail segment (e.g., distance from segment start). This allows for precise modeling of "linear" conflicts where agents are on the same edge but at different offsets.

For Mobile (d=2): λ=∅ (or strictly 0). The state is fully defined by (v,θ).

For Drone (d=3): λ∈{0,5,10,15,…} represents the discrete altitude layer in meters. This discretization is critical for the "Layered Airspace" model.

θ: The orientation, essential for non-holonomic constraints inherent in car-like mobile robots and rail switches.

2.3 Motion Semantics
The transitions between states must adhere to axioms that preserve the physical consistency of the system.

Axiom 2.1 (Dimensionality Conservation). An agent acts as a physically immutable entity regarding its operational dimension.

∀a∈A,∀t 
1
​
 ,t 
2
​
 ∈R 
≥0
​
 :κ(a,t 
1
​
 )=κ(a,t 
2
​
 )
This axiom prevents "mode-switching" errors in the planner, such as a ground robot attempting to traverse an aerial edge to bypass a blocked corridor.

Axiom 2.2 (Inter-Dimensional Transition). Interaction between dimensions is strictly local.

interact(a 
1
​
 ,a 
2
​
 ,v)⟹κ(a 
1
​
 )∈δ(v)∧κ(a 
2
​
 )∈δ(v)
This axiom dictates that a drone flying at 15m (λ=15) does not conflict with a ground robot (λ=0) at the same x,y coordinate unless the drone descends to a vertex v where the "Air" and "Ground" manifolds explicitly merge (e.g., a landing station). This formalizes the vertical separation necessary for safety and efficient planning.

3. Dimensional Conflict Decomposition
3.1 Motivation
The standard Conflict-Based Search (CBS) algorithm treats all conflicts uniformly as spatiotemporal overlaps, resolving them by imposing vertex or edge constraints. While complete and optimal for homogeneous grids, this approach is inefficient for MD-MAPF. The geometry of a conflict—and thus the search space of its resolution—is fundamentally determined by the kinematics of the agents involved. A head-on conflict between two rail robots on a single track implies a deadlock that requires one agent to retreat to a siding, potentially tens of meters away. This is structurally distinct from a planar conflict where a local swerve suffices. By treating these conflicts uniformly, standard CBS suffers from a massive branching factor as it attempts to resolve the rail deadlock via incremental waits.

We introduce Dimensional Conflict Decomposition, a taxonomy that categorizes conflicts based on the dimensional pair (κ(a 
i
​
 ),κ(a 
j
​
 )) and assigns specialized resolution strategies to each class.

3.2 Conflict Taxonomy
Definition 3.1 (Dimensional Conflict). Let a 
1
​
 ,a 
2
​
 ∈A be two conflicting agents. The conflict is classified into one of six classes based on their dimensionality pair (d 
1
​
 ,d 
2
​
 ):

3.3 Resolution Strategies
For each conflict class, we implement a specific resolve() function that generates constraints tailored to the kinematic reality of the agents.

Strategy 3.1 (LINEAR — C 
1
​
 ): Segment Locking

Context: Two rail robots meet on a bidirectional track segment S.

Resolution: Standard vertex constraints are insufficient because agents cannot pass. We apply a Segment Constraint. One agent must be forbidden from entering the entire segment S (and potentially the sequence of segments up to the next siding) until the other has cleared it.

Logic:

This effectively treats the rail segment as a critical section or a mutex resource, pruning the search tree of all invalid "waiting" states on the track itself.

Strategy 3.2 (PLANAR — C 
2
​
 ): Standard Splitting

Context: Two mobile robots collide at vertex v.

Resolution: We apply standard CBS splitting, potentially enhanced by Cardinal Conflict detection. If the conflict is cardinal (i.e., resolving it increases the cost of the path), it is prioritized in the high-level search to prove optimality faster.

Strategy 3.3 (CROSSING — C 
3
​
 ): Inertia-Based Priority

Context: A mobile robot crosses a rail track.

Resolution: Rail robots typically have higher inertia and braking distances. The resolution strategy implicitly prioritizes the rail agent by exploring the branch that constrains the mobile robot first (heuristic ordering).

Strategy 3.4 (AERIAL — C 
4
​
 ): Layered Separation

Context: Two drones conflict at the same altitude.

Resolution: Similar to Planar resolution, but constraints are applied to the 3D vertex (x,y,z). Additionally, "semi-cardinal" conflicts may be resolved by forcing one agent to change altitude layers if the horizontal heuristic allows, leveraging the sparse vertical dimension.

Strategy 3.5 (VERTICAL — C 
5
​
 ): Corridor Mutual Exclusion

Context: Two drones attempt to use the same vertical corridor (e.g., an elevator shaft or virtual ascent tube) simultaneously.

Resolution: Vertical corridors are treated as atomic resources with a capacity of 1. To prevent downwash effects and collision risk during ascent/descent, we enforce strictly sequential access.

This prevents the solver from generating dangerous plans where drones follow each other too closely in a vertical column.

Strategy 3.6 (AIR-GROUND — C 
6
​
 ): Asymmetric Handoff

Context: A drone lands on a pad occupied by a mobile robot.

Resolution: Drones have limited hovering time (battery constraint). Ground robots have effectively infinite wait time. The solver prioritizes the drone's landing window.

3.4 MIXED-CBS Algorithm
The MIXED-CBS algorithm integrates these decomposition strategies into the high-level search loop. By classifying the conflict C extracted from a node N, the algorithm dispatches the appropriate resolve function.

3.5 Theoretical Properties
Theorem 3.1 (Completeness). MIXED-CBS is complete: if a valid conflict-free solution exists for the MD-MAPF instance, the algorithm will find it. Proof Sketch: The Dimensional Conflict Decomposition acts as a constraint generation heuristic. Since each strategy (e.g., segment locking for rail) produces a subset of the valid constraint space, and the high-level search explores the tree of these constraints exhaustively (or until a solution is found), no valid solution is permanently pruned. The strategies merely guide the search away from invalid or inefficient subspaces (like infinite waits on a single track) more aggressively than generic collision checking.

Theorem 3.2 (Makespan Optimality). MIXED-CBS returns a solution with minimal makespan (or sum-of-costs, depending on the objective function). Proof Sketch: CBS is known to be optimal because it performs a Best-First Search on the constraint tree. The cost of a node is the sum of the costs of the individual paths. Since the conflict decomposition does not alter the cost calculation or the admissibility of the low-level A* heuristics, the first conflict-free node extracted from the OPEN list is guaranteed to be optimal.

4. Energy-Constrained CBS (E-CBS)
4.1 Motivation
While standard MAPF optimizes for time or distance, heterogeneous fleets—particularly those including electric drones—operate under strict energy budgets. A path may be spatially valid and optimal in terms of time, yet infeasible if it depletes the agent's battery mid-transit. Furthermore, simple pre-planning (charging before the mission) is insufficient for "Lifelong" scenarios where agents perform continuous cycles of tasks. The planner must inherently support Endogenous Task Generation—the ability to dynamically insert charging tasks into an agent's schedule when energy reserves are projected to fall below critical levels.

4.2 Energy Model
We integrate a continuous energy variable into the agent's state space.

Definition 4.1 (Drone Energy State). The state of an agent is augmented:

s=(v,t,λ,e)where e∈[0,E 
max
​
 ]
Here, e represents the remaining energy capacity (e.g., in Watt-hours).

Definition 4.2 (Energy Consumption). We define a consumption function consume that varies based on the agent's kinematic mode. This is crucial for drones, where vertical motion is significantly more expensive than horizontal motion.

consume(α,Δt,Δd)=P(α)⋅ 
3600
Δt
​
 
The power draw P(α) is mode-dependent:

Definition 4.3 (Path Energy Validity). A path π=⟨(v 
0
​
 ,t 
0
​
 ),…,(v 
k
​
 ,t 
k
​
 )⟩ is energy-valid if and only if:

Sustainability: ∀i∈[1,k]:e 
i
​
 =e 
i−1
​
 −consume(...)>0.

Recharge Logic: If v 
i
​
  is a designated charging station, the energy state resets: e 
i
​
 =E 
max
​
  after a mandatory service time duration.

4.3 Automatic Charging Station Insertion
Standard MAPF solvers treat the task sequence as immutable. E-CBS introduces a mechanism to modify the task sequence endogenously.

Definition 4.4 (Energy Violation). An energy violation is detected during the low-level search or high-level simulation. It is defined as a tuple ϵ=(a,t 
depleted
​
 ,v 
depleted
​
 ), indicating that agent a is projected to reach e=0 at state (v,t).

Algorithm 4.1 (Energy Violation Resolution). When an energy violation is detected for agent a, the solver does not immediately fail. Instead, it attempts to "repair" the plan by injecting a charging task.

This effectively converts the violation into a routing constraint. The high-level solver now manages the conflict between the agent's need to charge and the availability of the charging pads (which are treated as resources with capacity 1). If multiple agents need to charge simultaneously, E-CBS resolves the resulting spatial conflicts at the charging station vertices.

4.4 E-CBS Algorithm Structure
The E-CBS algorithm wraps the standard CBS loop. The critical modification is in the low-level solver (typically A* or SIPP), which must now prune states where g 
energy
​
 (n)>E 
max
​
 .

4.5 Separation of Horizontal and Vertical Consumption
Lemma 4.1. To ensure accurate energy estimation, the consumption model must separate velocity vectors. $$E_{total} = E_{horizontal} + E_{vertical}$$Using a simple Euclidean distance in 3D space would underestimate the cost of climbing. The specific cost function for a 3D move from (x 
1
​
 ,y 
1
​
 ,z 
1
​
 ) to (x 
2
​
 ,y 
2
​
 ,z 
2
​
 ) is:

E 
move
​
 = 
v 
xy
​
 
P 
horiz
​
 ⋅ 
Δx 
2
 +Δy 
2
 

​
 
​
 + 
v 
z
​
 
P 
vert
​
 ⋅∣Δz∣
​
 
This separation is crucial for the "Layered Airspace" model, as it heavily penalizes frequent layer changes, encouraging drones to stay in their assigned altitude layer.

5. Layered Airspace Model
5.1 Structure: Discretizing the Void
Free-flight 3D pathfinding is computationally expensive and prone to dense conflicts. To make the problem tractable and safe, we adopt a Discrete Layered Airspace model, aligned with concepts from Unmanned Traffic Management (UAV) research. This model structures the 3D void into a stack of 2D graphs connected by restricted transitions.

Definition 5.1 (Airspace Layer). The airspace Λ is stratified into discrete altitude bands:

Λ={λ 
0
​
 ,λ 
1
​
 ,λ 
2
​
 ,λ 
3
​
 }={0m,5m,10m,15m}
λ 
0
​
  (Ground): The interaction layer containing landing pads, charging stations, and ground obstacles.

λ 
1
​
  (Handoff/Buffer): A transition layer for approach and departure maneuvers.

λ 
2
​
  (Cruising A): A transit layer (e.g., dedicated to North/South traffic).

λ 
3
​
  (Cruising B): A transit layer (e.g., dedicated to East/West traffic).

Definition 5.2 (Vertical Corridor). Vertical movement is not allowed everywhere. We define specific Vertical Corridors (virtual elevator shafts) as the only vertices where layer changes are permitted.

C⊂V such that ∀v∈C,is_corridor(v)=true
This constraints reduces the dimensionality of the conflict detection problem. Instead of checking for 3D collisions everywhere, we mostly check for 2D collisions within layers and 1D collisions within corridors.

5.2 Transition Matrices
To enforce orderly traffic flow, we define a transition matrix T that dictates permissible moves between layers.

Definition 5.3 (Permitted Layer Transition).

T= 

​
  
1
1
0
0
​
  
1
1
1
0
​
  
0
1
1
1
​
  
0
0
1
1
​
  

​
 
(Rows/Cols correspond to λ 
0
​
 ,…,λ 
3
​
 ). A value of 1 indicates a valid move. The matrix structure implies that a drone cannot "skip" layers (e.g., jump from λ 
0
​
  to λ 
2
​
 ); it must traverse intermediate layers. This allows the planner to enforce "Ground Holding" strategies—keeping a drone on the ground (λ 
0
​
 ) if the transit layers (λ 
2
​
 ,λ 
3
​
 ) are congested, rather than having it hover efficiently in λ 
1
​
 .

5.3 Corridor Capacity and Mutex
Axiom 5.1 (Corridor Exclusivity). Vertical conflicts are particularly dangerous due to downwash physics. Therefore, we enforce a strict mutex on vertical corridors.

∀C∈Corridors,∀t:∣{a∈A∣position(a,t)∈C}∣≤1
This axiom dictates that only one drone can occupy a vertical corridor stack at any given time. If two drones need to ascend, they must do so sequentially. The high-level solver (MIXED-CBS) handles this via the Strategy 3.5 (VERTICAL) conflict resolution, effectively treating the corridor as a shared resource with capacity 1.

6. Hybrid Planning with Potential Fields
6.1 Concept: Bridging Planning and Execution
While CBS provides optimal, conflict-free plans in the discrete space-time graph, real-world execution is subject to continuous noise, sensor errors, and minor delays. A system that blindly follows the discrete plan may fail if an agent deviates slightly or if an unmapped dynamic obstacle appears. To address this, we implement a Hybrid Planning architecture.

Structure:

Global Planner (CBS): Generates the high-level sequence of waypoints and critical synchronization constraints (e.g., "Wait at vertex A until time t=10").

Local Executive (Potential Fields): A reactive controller that generates velocity commands to follow the global path while locally repelling from obstacles.

6.2 Field Definition
We utilize Artificial Potential Fields (APF) to guide the agents.

Definition 6.1 (Potential Field Function). The potential energy U(q) at configuration q is the sum of attractive and repulsive components:

U(q)=U 
att
​
 (q,q 
goal
​
 )+∑U 
rep
​
 (q,o 
i
​
 )
where q 
goal
​
  is the next immediate waypoint provided by the CBS planner.

Definition 6.2 (Modified Heuristic). To align the low-level A* search with the potential field reality, we modify the heuristic function h(v) used in the planner:

h 
′
 (v)=h(v)−α⋅Φ(v)
where Φ(v) is a "congestion potential" map learned from historical data or density estimates. This encourages the global planner to avoid regions that are typically crowded, effectively smoothing the execution phase.

6.3 Deviation Detection and Replanning
The hybrid system requires a mechanism to detect when the reactive layer (APF) has failed to track the global plan (e.g., getting stuck in a local minimum).

Definition 6.3 (Deviation Metric). The deviation of agent a at time t is:

dev(a,t)=∣∣pos 
actual
​
 (a,t)−pos 
planned
​
 (a,t)∣∣
Rule 6.1 (Local Replanning Trigger). If dev(a,t)>δ 
thresh
​
 , the agent triggers a local repair:

The agent pauses (enters "Wait" state).

A local A* search is triggered to find a path from pos 
actual
​
  back to the global path π 
CBS
​
  within a horizon H.

If local repair fails (e.g., due to a new blocked obstacle), a global replan request is sent to the central CBS solver.

This hierarchical approach ensures that the expensive global solver is only invoked when the lightweight local reactive layer cannot handle the disturbance.

7. Formal Definitions
To consolidate the proposed framework, we summarize the key formalisms used throughout this specification.

7.1 Notation
7.2 System Invariants
Invariant 7.1 (Conflict-Freedom). No two agents may occupy intersecting volumes at the same time.

∀a 
i
​
 ,a 
j
​
 ∈A,a 
i
​
 

=a 
j
​
 ,∀t:Vol(a 
i
​
 ,t)∩Vol(a 
j
​
 ,t)=∅
where Vol is determined by the agent's kinematic shape and dimensionality.

Invariant 7.2 (Energy Sustainability). All agents must maintain positive energy reserves or be located at a charging station.

∀a∈A,∀t:e 
a
​
 (t)>0∨is_charging(a,t)
Invariant 7.3 (Dimensional Consistency). An agent effectively exists only within its compatible manifold.

∀a∈A,∀t:κ(a)∈δ(position(a,t))
8. Conclusion
This report has formalized the algorithmic foundations of the MAPF-HET system, a comprehensive planning architecture designed for the coordination of mixed-dimensional heterogeneous fleets. By rigorously defining the MD-MAPF problem, we have extended the scope of Multi-Agent Path Finding beyond homogeneous grids to complex, multi-manifold environments representative of modern industry (e.g., ports, multi-modal warehouses).

The Dimensional Conflict Decomposition transforms the challenge of heterogeneity into an algorithmic advantage. By classifying conflicts into six distinct types (e.g., Linear, Vertical, Cross-Dimensional), we enable the use of specialized, efficient resolution strategies that significantly prune the CBS search tree compared to generic solvers.

The E-CBS algorithm introduces a necessary layer of realism by treating energy as a hard constraint and enabling the endogenous generation of charging tasks. This ensures that generated plans are not just collision-free but mission-feasible. The Layered Airspace model and Hybrid Planning architecture further enhance robustness, providing structured mechanisms to handle 3D complexity and real-world execution noise, respectively.

Collectively, these contributions provide a theoretically sound and practically viable framework for the next generation of large-scale, heterogeneous autonomous systems.

Implementation References
Core Solvers: internal/algo/mixed_cbs.go (MIXED-CBS), internal/algo/energy_cbs.go (E-CBS).

Low-Level Planners: internal/algo/astar3d.go (3D Energy-A*), internal/algo/sipp_rail.go (1D SIPP for Rail).

Models: internal/core/airspace.go (Layered Graph), internal/core/energy_model.go (Battery physics).

Execution: internal/algo/hybrid_cbs.go (Hybrid executive), internal/algo/potential_field.go (APF Controller).

Document prepared for internal documentation of the Elektrokombinacija project.


ijcai.org
Scalable Mechanism Design for Multi-Agent Path Finding - IJCAI
Opens in a new window

ijcai.org
Multi-Agent Pathfinding with Continuous Time - IJCAI
Opens in a new window

diva-portal.org
Multi-agent Path Planning Based on Conflict-Based Search (CBS) Variations for Heterogeneous Robots - Diva-Portal.org
Opens in a new window

arxiv.org
db-CBS: Discontinuity-Bounded Conflict-Based Search for Multi-Robot Kinodynamic Motion Planning - arXiv
Opens in a new window

mdpi.com
A Method for Orderly and Parallel Planning of Public Route Networks for Logistics Based on Urban Low-Altitude Digital Airspace Environment Risks - MDPI
Opens in a new window

arxiv.org
[2011.00441] CL-MAPF: Multi-Agent Path Finding for Car-Like Robots with Kinematic and Spatiotemporal Constraints - arXiv
Opens in a new window

dspace.mit.edu
Towards Learning-guided Search for Coordination of Multi-agent Transportation at Scale - DSpace@MIT
Opens in a new window

ojs.aaai.org
Bidirectional Temporal Plan Graph: Enabling Switchable Passing Orders for More Efficient Multi-Agent Path Finding Plan Execution
Opens in a new window

arxiv.org
Bidirectional Temporal Plan Graph: Enabling Switchable Passing Orders for More Efficient Multi-Agent Path Finding Plan Execution - arXiv
Opens in a new window

cdn.aaai.org
Scalable Rail Planning and Replanning: Winning the 2020 Flatland Challenge - AAAI
Opens in a new window

arxiv.org
Hierarchical Low-Altitude Wireless Network Empowered Air Traffic Management - arXiv
Opens in a new window

mdpi.com
Operational Performance of a 3D Urban Aerial Network and Agent-Distributed Architecture for Freight Delivery by Drones - MDPI
Opens in a new window

emerald.com
Scheduling mobile robots in dynamic production environments considering battery management constraints | Logistics Research | Emerald Publishing
Opens in a new window

mdpi.com
Optimization of Orderly-Charging Strategy of Multi-Zone Electric Vehicle Based on Reinforcement Learning - MDPI
Opens in a new window

researchgate.net
Multi-Agent Systems Coverage Control in Mixed-Dimensional and Hybrid Environments | Request PDF - ResearchGate
Opens in a new window

arxiv.org
Multi-Agent Path Finding Using Conflict-Based Search and Structural-Semantic Topometric Maps - arXiv
Opens in a new window

mdpi.com
Extending Conflict-Based Search for Optimal and Efficient Quadrotor Swarm Motion Planning - MDPI
Opens in a new window

mdpi.com
Optimization of a Navigation System for Autonomous Charging of Intelligent Vehicles Based on the Bidirectional A* Algorithm and YOLOv11n Model - MDPI
Opens in a new window

researchgate.net
Charging Station Placement for Limited Energy Robots - ResearchGate
Opens in a new window

researchgate.net
Air Corridor Planning for Urban Drone Delivery: Complexity Analysis and Comparison via Multi-Commodity Network Flow and Graph Search | Request PDF - ResearchGate
Opens in a new window

cdn.aaai.org
Adding Heuristics to Conflict-Based Search for Multi-Agent Path Finding - Association for the Advancement of Artificial Intelligence (AAAI)
Opens in a new window

cdn.aaai.org
Spatially Distributed Multiagent Path Planning - Association for the Advancement of Artificial Intelligence (AAAI)
Opens in a new window

techscience.com
An Improved Bounded Conflict-Based Search for Multi-AGV Pathfinding in Automated Container Terminals - Tech Science Press
Opens in a new window

mdpi.com
Hybrid Path Planning Algorithm for Autonomous Mobile Robots: A Comprehensive Review
Opens in a new window

mdpi.com
Conflict-Free 3D Path Planning for Multi-UAV Based on Jump Point Search and Incremental Update - MDPI
Opens in a new window

ifaamas.csc.liv.ac.uk
Enhancing Lifelong Multi-Agent Path-finding by Using Artificial Potential Fields - IFAAMAS
Opens in a new window

researchgate.net
Enhancing Lifelong Multi-Agent Path-finding by Using Artificial Potential Fields
Opens in a new window

ieeexplore.ieee.org
A Hybrid Path Planning Method Based on Improved A∗ and CSA-APF Algorithms - IEEE Xplore
Opens in a new window

arxiv.org
Path Planning with Potential Field-Based Obstacle Avoidance in a 3D Environment by an Unmanned Aerial Vehicle - arXiv
Opens in a new window

themoonlight.io
[Literature Review] Enhancing Lifelong Multi-Agent Path-finding by Using Artificial Potential Fields - Moonlight