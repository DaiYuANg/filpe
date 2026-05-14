package discovery

import "encoding/json"

type delegate struct {
	meta nodeMeta
}

func (d *delegate) NodeMeta(limit int) []byte {
	payload, err := json.Marshal(d.meta)
	if err != nil || len(payload) > limit {
		return nil
	}
	return payload
}

func (d *delegate) NotifyMsg([]byte) {}

func (d *delegate) GetBroadcasts(_, _ int) [][]byte {
	return nil
}

func (d *delegate) LocalState(bool) []byte {
	return d.NodeMeta(512)
}

func (d *delegate) MergeRemoteState([]byte, bool) {}
