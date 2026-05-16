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
		strings.Trim(defaultSearchPath, "/"):          func() { s.handleSearch(w, r) },
		strings.Trim(defaultClusterMembersPath, "/"):  func() { s.handleClusterMembers(w, r) },
		strings.Trim(defaultClusterBootstrapPath, "/"): func() { s.handleClusterBootstrap(w, r) },
		strings.Trim(defaultClusterJoinPath, "/"):     func() { s.handleClusterJoin(w, r) },
		strings.Trim(defaultDiscoveryPath, "/"):       func() { s.handleDiscovery(w, r) },
	}
	if routeHandler, ok := routes[route]; ok {
		routeHandler()
		return true
	}
	if s.handleStorageShardRoute(w, r, parts) {
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

