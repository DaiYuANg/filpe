package handler

import (
	"net/http"
	"strings"
)

func (s *Service) handleControlRoute(w http.ResponseWriter, r *http.Request, route string, parts []string) bool {
	switch {
	case isHealthRoute(route):
		s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return true
	case s.handleS3Route(w, r):
		return true
	}
	return s.handleNamedControlRoute(w, r, route, parts)
}

func (s *Service) handleNamedControlRoute(w http.ResponseWriter, r *http.Request, route string, parts []string) bool {
	routes := map[string]func(){
		strings.Trim(defaultSearchPath, "/"):           func() { s.handleSearch(w, r) },
		strings.Trim(defaultClusterMembersPath, "/"):   func() { s.handleClusterMembers(w, r) },
		strings.Trim(defaultClusterBootstrapPath, "/"): func() { s.handleClusterBootstrap(w, r) },
		strings.Trim(defaultClusterJoinPath, "/"):      func() { s.handleClusterJoin(w, r) },
		strings.Trim(defaultClusterRebalancePath, "/"): func() { s.handleClusterRebalance(w, r) },
		strings.Trim(defaultClusterRebalancePlanPath, "/"): func() {
			s.handleClusterRebalancePlan(w, r)
		},
		strings.Trim(defaultClusterStorageNodesPath, "/"): func() {
			s.handleClusterStorageNodes(w, r)
		},
		strings.Trim(defaultDiscoveryPath, "/"):    func() { s.handleDiscovery(w, r) },
		strings.Trim(defaultRepairStatusPath, "/"): func() { s.handleRepairStatus(w, r) },
	}
	if routeHandler, ok := routes[route]; ok {
		routeHandler()
		return true
	}
	if s.handleStorageShardRoute(w, r, parts) {
		return true
	}
	if isClusterMemberActionRoute(parts) {
		s.handleClusterMemberAction(w, r, parts[2], parts[3])
		return true
	}
	if isClusterMemberRoute(parts) {
		s.handleClusterMember(w, r, parts[2])
		return true
	}
	return false
}

func isHealthRoute(route string) bool {
	return route == "healthz" || route == "health"
}

func isClusterMemberRoute(parts []string) bool {
	return len(parts) == 3 && parts[0] == "_cluster" && parts[1] == "members"
}

func isClusterMemberActionRoute(parts []string) bool {
	return len(parts) == 4 && parts[0] == "_cluster" && parts[1] == "members"
}
