package engine

type StorageNodeInfo struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Local   bool   `json:"local"`
	Drained bool   `json:"drained"`
}

func (e *Engine) StorageNodeInfos() []StorageNodeInfo {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	nodes := e.storageNodesLocked()
	infos := make([]StorageNodeInfo, 0, len(nodes))
	for _, node := range nodes {
		nodeID := node.ID()
		_, drained := e.drainedNodes[nodeID]
		infos = append(infos, StorageNodeInfo{
			ID:      nodeID,
			Address: node.Address(),
			Local:   nodeID == e.localNodeID,
			Drained: drained,
		})
	}
	return infos
}
