package raft

func (rt *Runtime) LocalReplicaID() uint64 {
	if rt == nil || rt.cfg == nil {
		return 0
	}
	return rt.cfg.replicaID
}

func (rt *Runtime) LocalRaftAddress() string {
	if rt == nil || rt.cfg == nil {
		return ""
	}
	return rt.cfg.raftAddress
}
