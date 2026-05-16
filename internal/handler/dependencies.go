package handler

import (
	"github.com/lyonbrown4d/maxio/internal/discovery"
	"github.com/lyonbrown4d/maxio/internal/engine"
	raftx "github.com/lyonbrown4d/maxio/internal/raft"
	"github.com/lyonbrown4d/maxio/internal/repair"
	maxios3 "github.com/lyonbrown4d/maxio/internal/s3"
	"github.com/lyonbrown4d/maxio/object"
)

// Dependencies groups handler dependencies to keep dix providers shallow.
type Dependencies struct {
	objects   *object.Service
	engine    *engine.Engine
	raft      *raftx.Runtime
	discovery *discovery.Runtime
	s3        *maxios3.Service
	repair    *repair.Runtime
}

// NewDependencies wires the handler dependency set.
func NewDependencies(
	objects *object.Service,
	engineStore *engine.Engine,
	raftRuntime *raftx.Runtime,
	discoveryRuntime *discovery.Runtime,
	s3Service *maxios3.Service,
	repairRuntime *repair.Runtime,
) Dependencies {
	return Dependencies{
		objects:   objects,
		engine:    engineStore,
		raft:      raftRuntime,
		discovery: discoveryRuntime,
		s3:        s3Service,
		repair:    repairRuntime,
	}
}
