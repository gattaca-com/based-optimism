package params

// CHANGE(thedevbirb): A global variable set only at node startup that assess
// whether the node is running in chain replication mode, leading to some
// syncing and safety functionality to be disabled.
var BopReplay = false
