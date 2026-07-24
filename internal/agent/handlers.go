package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/nixomose/zfstool/internal/api"
	"github.com/nixomose/zfstool/internal/collector"
	"github.com/nixomose/zfstool/internal/version"
	"github.com/nixomose/zfstool/internal/zfsname"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, api.ErrorBody{Error: msg, Code: http.StatusText(status)})
}

// writeClientErr maps validation errors to 400; returns true if handled.
func writeClientErr(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, collector.ErrInvalidDevice) || errors.Is(err, zfsname.ErrInvalid) || errors.Is(err, collector.ErrInvalidBrowse) {
		writeErr(w, http.StatusBadRequest, err.Error())
		return true
	}
	return false
}

func (s *Server) handleHost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h, err := collector.CollectHost(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := api.HostInfo{
		Hostname:       h.Hostname,
		OSName:         h.OSName,
		OSVersion:      h.OSVersion,
		Kernel:         h.Kernel,
		Arch:           h.Arch,
		UptimeSeconds:  h.UptimeSeconds,
		MemTotalKB:     h.MemTotalKB,
		MemAvailableKB: h.MemAvailableKB,
		ZFSVersions:    h.ZFSVersions,
		ZFSMismatch:    h.ZFSMismatch,
		ARC:            h.ARC,
		AgentVersion:   version.Version,
		CollectedAt:    time.Now().UTC(),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePools(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pools, err := collector.ListPools(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pools)
}

func (s *Server) handlePoolStatus(w http.ResponseWriter, r *http.Request) {
	pool := r.PathValue("pool")
	st, err := collector.PoolStatusFull(r.Context(), pool)
	if err != nil {
		if writeClientErr(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handlePoolProps(w http.ResponseWriter, r *http.Request) {
	pool := r.PathValue("pool")
	props, src, err := collector.PoolProperties(r.Context(), pool)
	if err != nil {
		if writeClientErr(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"properties": props, "source": src})
}

func (s *Server) handlePoolHistory(w http.ResponseWriter, r *http.Request) {
	pool := r.PathValue("pool")
	off, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	lim, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if lim <= 0 {
		lim = 500
	}
	entries, err := collector.PoolHistory(r.Context(), pool, off, lim)
	if err != nil {
		if writeClientErr(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handlePoolMaintenance(w http.ResponseWriter, r *http.Request) {
	pool := r.PathValue("pool")
	mb, err := collector.Maintenance(r.Context(), pool)
	if err != nil {
		if writeClientErr(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mb)
}

func (s *Server) handleDatasets(w http.ResponseWriter, r *http.Request) {
	pool := r.URL.Query().Get("pool")
	rows, err := collector.ListDatasets(r.Context(), pool)
	if err != nil {
		if writeClientErr(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) handleDatasetProps(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}
	props, src, err := collector.GetDatasetProperties(r.Context(), name)
	if err != nil {
		if writeClientErr(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"name": name, "properties": props, "source": src})
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	ds := r.URL.Query().Get("dataset")
	if ds == "" {
		writeErr(w, http.StatusBadRequest, "dataset required")
		return
	}
	path := r.URL.Query().Get("path")
	res, err := collector.BrowseDir(r.Context(), ds, path)
	if err != nil {
		if writeClientErr(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleBookmarks(w http.ResponseWriter, r *http.Request) {
	pool := r.URL.Query().Get("pool")
	rows, err := collector.ListBookmarks(r.Context(), pool)
	if err != nil {
		if writeClientErr(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) handleHolds(w http.ResponseWriter, r *http.Request) {
	snap := r.URL.Query().Get("snapshot")
	if snap == "" {
		writeErr(w, http.StatusBadRequest, "snapshot required")
		return
	}
	h, err := collector.Holds(r.Context(), snap)
	if err != nil {
		if writeClientErr(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, h)
}

func (s *Server) handleIOStat(w http.ResponseWriter, r *http.Request) {
	samples, err := collector.IOStatOneShot(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, samples)
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	pool := r.URL.Query().Get("pool")
	if pool == "" {
		writeErr(w, http.StatusBadRequest, "pool required")
		return
	}
	g, err := collector.BuildDatasetGraph(r.Context(), pool)
	if err != nil {
		if writeClientErr(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleKernelLog(w http.ResponseWriter, r *http.Request) {
	pool := r.URL.Query().Get("pool")
	lines, err := collector.KernelLogSnippet(r.Context(), pool, 80)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, lines)
}

func (s *Server) handleModuleParams(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, collector.ModuleParams())
}

func (s *Server) handleZfsAllow(w http.ResponseWriter, r *http.Request) {
	ds := r.URL.Query().Get("dataset")
	if ds == "" {
		writeErr(w, http.StatusBadRequest, "dataset required")
		return
	}
	out, err := collector.ZfsAllowOutput(r.Context(), ds)
	if err != nil {
		if writeClientErr(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"output": out})
}

func (s *Server) handleDisks(w http.ResponseWriter, r *http.Request) {
	disks, err := collector.ListDisks(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, disks)
}

func (s *Server) handleSmart(w http.ResponseWriter, r *http.Request) {
	dev := r.PathValue("dev")
	if dev == "" {
		writeErr(w, http.StatusBadRequest, "device required")
		return
	}
	safe, err := collector.ValidateBlockDevicePath(dev)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	d, err := collector.SMARTJSON(r.Context(), safe)
	if err != nil {
		if writeClientErr(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handlePoolDevices(w http.ResponseWriter, r *http.Request) {
	pool := r.PathValue("pool")
	st, err := collector.PoolStatusFull(r.Context(), pool)
	if err != nil {
		if writeClientErr(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	devs := collector.MapVDevToDevices(st.Config)
	writeJSON(w, http.StatusOK, devs)
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": version.Version})
}

// ZfsDiff runs zfs diff (bounded) — POST body JSON {from,to}
func (s *Server) handleZfsDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var body struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	out, err := collector.ZfsDiff(ctx, body.From, body.To)
	if err != nil {
		if writeClientErr(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"output": out})
}
