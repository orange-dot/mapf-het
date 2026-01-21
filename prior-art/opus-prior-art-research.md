# Prior Art Investigation for MAPF-HET: Assessment of Novelty Claims

A comprehensive investigation across academic literature, patents, and commercial implementations reveals **strong novelty support for all four algorithmic contributions**. While related work exists in each area, the specific formulations proposed—particularly the dimensionality function κ(a), dimensional conflict taxonomy, automatic charging waypoint insertion, and functional layer naming—appear to be genuine extensions to the MAPF literature.

## Mixed-Dimensional MAPF represents a genuine formalization gap

The investigation found no existing work that explicitly models agents operating in fundamentally different dimensional spaces (1D rail, 2D ground, 3D aerial) within a unified MAPF formulation. The closest prior work is **G-MAPF** (Atzmon et al., SoCS 2020), which generalizes heterogeneity for varying sizes, shapes, and behaviors but assumes all agents operate on the same graph structure. Similarly, **LA-MAPF** (Li et al., AAAI 2019) handles geometric agents with varying footprints but not dimensional restrictions.

Key prior art citations establish the gap:

- **G-MAPF** introduces state-based agent representation with transition functions but lacks dimensionality function κ(a) → {1,2,3}
- **Multi-Train Path Finding** (Atzmon et al., SoCS 2019) addresses 1D rail networks in isolation, without integration with 2D/3D agents
- **Coordinated MAPF for Drones and Trucks** (arXiv 2021) comes closest by combining ground and aerial agents, but uses sequential two-stage planning rather than unified formulation, assumes shared road graphs, and lacks the 1D rail component
- **HEHA** (arXiv 2024) coordinates drones, wheeled, and legged robots but uses hierarchical task-level coordination with separate motion planners per robot type

The **novelty claim** can be positioned around three elements absent from all prior work: the dimensionality function κ(a) mapping agents to dimensional spaces, the vertex compatibility function δ(v) defining dimension-dependent access, and inter-dimensional transition modeling within a single MAPF graph. This advances G-MAPF toward dimensional diversity rather than just geometric/kinematic heterogeneity.

## Dimensional Conflict Decomposition fills a classification void

Existing conflict classification schemes in MAPF literature are based on three distinct approaches—none addressing dimensional interaction. The foundational **CBS paper** (Sharon et al., AIJ 2015) defines only vertex and edge conflicts without structural distinction. **ICBS** (Boyarski et al., IJCAI 2015) introduced cardinal/semi-cardinal/non-cardinal classification based on cost impact using MDDs. The most sophisticated existing framework is **Pairwise Symmetry Reasoning** (Li et al., AIJ 2021), which recognizes corridor, rectangle, and target symmetry patterns—achieving up to **4 orders of magnitude** runtime reduction.

The proposed six-class taxonomy (LINEAR, PLANAR, CROSSING, AERIAL, VERTICAL, AIR-GROUND) based on dimensional interaction represents a fundamentally different classification principle:

| Proposed Class | Closest Prior Art | Gap |
|----------------|-------------------|-----|
| LINEAR (1D-1D) | Corridor symmetry | Not formalized as dimensional |
| PLANAR (2D-2D) | Rectangle symmetry | Pattern-based, not dimensional |
| CROSSING (1D-2D) | None | Completely novel |
| AERIAL (3D-3D same layer) | Limited 3D MAPF work | No conflict taxonomy exists |
| VERTICAL (3D-3D layers) | None | Completely novel |
| AIR-GROUND (3D-1D/3D-2D) | Drone-truck work | No formalized conflict types |

**Dimension-specific constraint generation strategies** exploiting problem structure are novel. The closest related work—symmetry reasoning by Li, Harabor, and Koenig—focuses on geometric patterns rather than dimensional characteristics.

## Energy-Constrained CBS addresses integration gap in existing approaches

Energy-aware planning is well-studied in vehicle routing problems, but CBS-integrated approaches with automatic charging station waypoint insertion are absent. The closest work is **NRHF-MAPF** (Scott et al., IEEE 2024), which extends CBS for hybrid-fuel UAVs with battery and generator constraints plus noise-restricted zones. However, this focuses on noise restrictions rather than autonomous charging, uses hybrid fuel models, and does not include automatic waypoint insertion when energy violations are detected.

Related work forms three categories with distinct gaps:

**CBS-related approaches** include NRHF-MAPF (noise + hybrid fuel focus), Multi-Objective CBS (Ren et al., AAAI 2021) treating energy as optimization objective rather than hard constraint, and Energy-Constrained Multi-Task MAPD addressing task allocation rather than path-finding.

**Single-agent UAV planning** offers robust energy models but lacks multi-agent conflict resolution. Notable work includes Alyassi et al. (IEEE TASE 2022) on autonomous recharging with ML-based energy estimation, and Nekovář et al. (IEEE RAL 2024) on energy-constrained multi-UAV coverage.

**Electric Vehicle Routing Problem** literature (EVRPTW, GVRP, FCMVRP) provides mature charging station integration but operates in VRP frameworks without collision avoidance between vehicles.

E-CBS novelty claims rest on four elements: treating battery constraints as hard limits (versus optimization objectives), automatic charging station waypoint insertion during conflict resolution, action-specific drone energy models differentiating hover/move/climb/descend, and unified CBS-integrated approach for drone fleet coordination. The action-specific energy model appears particularly underexplored in existing multi-agent CBS variants.

## Layered Airspace Model refines existing multilayer concepts

This contribution has the most significant prior art, though key novel elements remain. **Labib et al.** (Sensors, 2019) presents a "Multilayer Low-Altitude Airspace Model" dividing Class G airspace (0-700 ft) into horizontal layers with inter-layer transitions at nodes. The **METROPOLIS Project** (Sunil et al., ATM 2015) tested layers segmented by travel direction, while **Liu et al.** (Wayne State/AIAA) describes altitude layer reservations requiring intermediate layer availability for transitions.

The proposed model's distinctive elements include:

**Functional layer naming** (ground/handoff/work/transit at 0m, 5m, 10m, 15m) appears novel—prior work uses direction-based or unnamed layers. This operational semantics provides specific purpose-driven airspace organization not found in METROPOLIS's heading-based layers or Labib's unnamed altitude bands.

**Vertical corridors as exclusive transition points** extends beyond prior work. Existing approaches use nodes or intersection points for transitions, but the exclusive corridor concept restricting all layer changes to defined vertical passages is a novel specification.

**Layer transition matrix restricting direct jumps** has partial precedent in Liu et al.'s requirement that intermediate layers be reserved, but the explicit matrix formalization within MAPF appears new.

UTM/U-space frameworks (NASA UTM ConOps, SESAR U-space, FAA UAM ConOps) provide high-level operational structures but do not specify internal physical layering. The proposed model provides specific structure that these frameworks leave to implementation.

## Commercial systems and patent landscape

The investigation identified several relevant patents with potential freedom-to-operate implications:

**US Patent 9,733,646B1** (Google/Intrinsic Innovation LLC, 2017) describes heterogeneous fleet coordination with mobile robotic devices and fixed manipulators, including battery exchange stations. This patent explicitly addresses different robot types (AGVs, pedestal robots, fork trucks) coordinated through central control and represents the most direct overlap with MAPF-HET concepts.

**US Patent 6,950,722** (Kiva Systems/Amazon, 2002) covers the foundational goods-to-person warehouse automation paradigm but focuses on homogeneous pod-carrying robots rather than true heterogeneous coordination.

Commercial warehouse systems demonstrate heterogeneous fleet management in practice. **Amazon Robotics** operates Proteus, Titan, Hercules, Sequoia, Cardinal, and other robot types through unified planning, using hybrid methods combining fast single-robot planning with fleet-wide coordination. Their **DeepFleet** foundation model coordinates over 1 million robots. **Geek+** explicitly supports "various navigation types to run as a combination in the same map" through their Robot Management System. **Locus Robotics** coordinates Origin, Vector, and Array robots through their LocusONE platform with task interleaving.

However, these commercial systems differ from MAPF-HET in critical ways: they primarily use heuristic-based approaches without formal optimality guarantees, handle heterogeneity through separate planning modules rather than unified formulations, and don't address the 1D/2D/3D dimensional distinctions central to MD-MAPF.

**Ocado Technology** merits special note for their battery swap automation—sub-one-minute hot swaps every ~2 hours with 12-minute charge cycles, achieving 7%+ increased robot utilization. This demonstrates commercial viability of automated energy management but operates within their proprietary grid system rather than general MAPF frameworks.

## Recommended positioning for novelty claims

**Contribution 1 (MD-MAPF)**: Position as extending G-MAPF toward dimensional diversity. Emphasize that no prior work provides (a) dimensionality function κ(a), (b) vertex compatibility δ(v), or (c) inter-dimensional transition modeling. Cite Atzmon et al. (G-MAPF), Li et al. (LA-MAPF), and the drone-truck coordination work to establish the gap.

**Contribution 2 (Dimensional Conflict Decomposition)**: Position as a fundamentally different classification principle from cost-based (ICBS) or pattern-based (symmetry reasoning) approaches. Emphasize CROSSING, VERTICAL, and AIR-GROUND classes as completely novel conflict types. Cite Li et al. (AIJ 2021) symmetry reasoning as closest but distinct work.

**Contribution 3 (E-CBS)**: Position as the first CBS variant with automatic charging waypoint insertion and action-specific drone energy models. Differentiate from NRHF-MAPF (noise restrictions, hybrid fuel) and MO-CBS (energy as objective, not constraint). Cite the EVRP literature to acknowledge the broader energy-constrained planning context.

**Contribution 4 (Layered Airspace Model)**: Acknowledge Labib et al. (2019) as closest prior art for multilayer structure. Differentiate through functional layer naming, exclusive vertical corridors, and integration with MAPF for collision-free path planning. Position as application-specific refinement optimized for multi-agent coordination rather than fundamentally new architecture.

## Essential citations by contribution

For **MD-MAPF**, the key references are Sharon et al. (CBS, AIJ 2015), Atzmon et al. (G-MAPF, SoCS 2020), Li et al. (LA-MAPF, AAAI 2019), and the Flatland Challenge work (Li et al., ICAPS 2021) for rail planning.

For **Dimensional Conflict Decomposition**, cite the CBS progression: Sharon et al. (original CBS), Boyarski et al. (ICBS, IJCAI 2015), Felner et al. (CBSH, ICAPS 2018), and Li et al. (symmetry reasoning, AIJ 2021). Also cite Andreychuk et al. (CCBS, AIJ 2022) for continuous-time extension.

For **E-CBS**, cite Scott et al. (NRHF-MAPF, 2024), Ren et al. (MO-CBS, AAAI 2021), Schneider et al. (EVRPTW, Transportation Science 2014), and Alyassi et al. (drone recharging, IEEE TASE 2022).

For **Layered Airspace Model**, cite Labib et al. (Sensors 2019), Sunil et al./METROPOLIS (ATM 2015), NASA UTM ConOps (2020), and FAA UAM ConOps (2023) for corridor concepts.

## Conclusion

This prior art investigation supports strong novelty claims for all four MAPF-HET contributions. The MD-MAPF formulation addresses a genuine gap in handling dimensional heterogeneity. The dimensional conflict taxonomy introduces a classification principle absent from existing literature. E-CBS fills an integration gap between CBS-based planning and energy constraints with automatic charging. The Layered Airspace Model, while building on existing multilayer concepts, introduces novel functional naming and exclusive vertical corridor specifications. 

The primary freedom-to-operate concern is **US Patent 9,733,646B1** for heterogeneous fleet coordination, which warrants detailed claims analysis before commercialization. For academic publication, all four contributions appear positioned to make defensible novelty claims with appropriate acknowledgment of the prior art identified in this investigation.