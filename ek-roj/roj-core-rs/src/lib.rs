//! ROJ Core Library
//!
//! Distributed consensus protocol for modular EV charging infrastructure.
//! Implements k-threshold voting with mDNS discovery and UDP transport.

pub mod types;
pub mod discovery;
pub mod transport;
pub mod consensus;

pub use types::*;
pub use discovery::Discovery;
pub use transport::Transport;
pub use consensus::Consensus;
