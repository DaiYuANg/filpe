package raft

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync/atomic"

	dbsm "github.com/lni/dragonboat/v4/statemachine"
)

const raftLookupKey = "state"

type raftStateMachine struct {
	shardID  uint64
	replica  uint64
	counter  atomic.Uint64
	disabled atomic.Bool
}

func newRaftStateMachine(shardID, replicaID uint64) dbsm.IStateMachine {
	return &raftStateMachine{
		shardID: shardID,
		replica: replicaID,
	}
}

func (s *raftStateMachine) Update(_ dbsm.Entry) (dbsm.Result, error) {
	if s == nil || s.disabled.Load() {
		return dbsm.Result{}, errors.New("raft state machine stopped")
	}
	return dbsm.Result{
		Value: s.counter.Add(1),
	}, nil
}

func (s *raftStateMachine) Lookup(input interface{}) (interface{}, error) {
	if s == nil {
		return nil, errors.New("raft state machine is nil")
	}
	key, ok := input.(string)
	if !ok {
		return nil, errors.New("unsupported lookup query")
	}
	switch {
	case key == raftLookupKey:
		return map[string]any{
			"shard_id":  s.shardID,
			"replica_id": s.replica,
			"applied":    s.counter.Load(),
		}, nil
	case strings.EqualFold(key, "health"):
		return "ok", nil
	default:
		return nil, errors.New("unsupported lookup key")
	}
}

func (s *raftStateMachine) SaveSnapshot(w io.Writer, _ dbsm.ISnapshotFileCollection, _ <-chan struct{}) error {
	if s == nil {
		return errors.New("raft state machine is nil")
	}
	encoded, err := json.Marshal(map[string]uint64{
		"applied": s.counter.Load(),
	})
	if err != nil {
		return err
	}
	_, err = w.Write(encoded)
	return err
}

func (s *raftStateMachine) RecoverFromSnapshot(r io.Reader, _ []dbsm.SnapshotFile, _ <-chan struct{}) error {
	if s == nil {
		return errors.New("raft state machine is nil")
	}
	var snapshot struct {
		Applied uint64 `json:"applied"`
	}
	if err := json.NewDecoder(r).Decode(&snapshot); err != nil {
		return err
	}
	s.counter.Store(snapshot.Applied)
	return nil
}

func (s *raftStateMachine) Close() error {
	if s == nil {
		return nil
	}
	s.disabled.Store(true)
	return nil
}

