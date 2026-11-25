package renderapi

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

const (
	ErrorCodePortLimitExceeded = "port_limit_exceeded"
	SandboxPortLimit           = 10
)

func exposedPort(sandboxID string, port int) sandboxesclient.SandboxExposedPort {
	return sandboxesclient.SandboxExposedPort{
		Protocol: sandboxesclient.Tcp,
		Port:     port,
		Url:      fmt.Sprintf("https://%s--%d.onrender.run", sandboxID, port),
	}
}

// ExposePorts seeds exposed ports on a sandbox.
func (r *SandboxResource) ExposePorts(sandboxID string, ports ...int) {
	if r.ports == nil {
		r.ports = map[string]sandboxesclient.SandboxExposedPorts{}
	}
	for _, port := range ports {
		r.ports[sandboxID] = append(r.ports[sandboxID], exposedPort(sandboxID, port))
	}
}

// Ports returns the ports exposed on a sandbox.
func (r *SandboxResource) Ports(sandboxID string) sandboxesclient.SandboxExposedPorts {
	return r.ports[sandboxID]
}

func (r *SandboxResource) portIndex(sandboxID string, port int) int {
	return slices.IndexFunc(r.ports[sandboxID], func(p sandboxesclient.SandboxExposedPort) bool {
		return p.Port == port
	})
}

func registerSandboxPortRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	sandboxFor := func(w http.ResponseWriter, r *http.Request) (*sandboxesclient.Sandbox, bool) {
		sb, found := s.sandboxByID(r.PathValue("sandboxId"))
		if !found {
			writeAPIError(w, http.StatusNotFound, "sandbox not found", "")
		}
		return sb, found
	}
	portFor := func(w http.ResponseWriter, r *http.Request) (int, bool) {
		port, err := strconv.Atoi(r.PathValue("port"))
		if err != nil || port < 1 || port > 65535 {
			writeAPIError(w, http.StatusBadRequest, "port must be between 1 and 65535", "")
			return 0, false
		}
		return port, true
	}

	mux.HandleFunc("GET /sandboxes/{sandboxId}/ports", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		sb, ok := sandboxFor(w, r)
		if !ok {
			return
		}

		ports := s.Sandboxes.Ports(sb.Id)
		if ports == nil {
			ports = sandboxesclient.SandboxExposedPorts{}
		}
		writeJSON(w, http.StatusOK, ports)
	})

	mux.HandleFunc("PUT /sandboxes/{sandboxId}/ports/{port}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		sb, ok := sandboxFor(w, r)
		if !ok {
			return
		}
		port, ok := portFor(w, r)
		if !ok {
			return
		}

		if i := s.Sandboxes.portIndex(sb.Id, port); i >= 0 {
			writeJSON(w, http.StatusOK, s.Sandboxes.ports[sb.Id][i])
			return
		}

		usage := len(s.Sandboxes.Ports(sb.Id))
		if usage >= SandboxPortLimit {
			writeJSON(w, http.StatusConflict, sandboxesclient.SandboxPortLimitError{
				Code:    ErrorCodePortLimitExceeded,
				Message: fmt.Sprintf("sandbox can expose at most %d ports", SandboxPortLimit),
				Limit:   SandboxPortLimit,
				Usage:   usage,
			})
			return
		}

		s.Sandboxes.ExposePorts(sb.Id, port)
		writeJSON(w, http.StatusOK, exposedPort(sb.Id, port))
	})

	mux.HandleFunc("DELETE /sandboxes/{sandboxId}/ports/{port}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		sb, ok := sandboxFor(w, r)
		if !ok {
			return
		}
		port, ok := portFor(w, r)
		if !ok {
			return
		}

		if i := s.Sandboxes.portIndex(sb.Id, port); i >= 0 {
			s.Sandboxes.ports[sb.Id] = slices.Delete(s.Sandboxes.ports[sb.Id], i, i+1)
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
