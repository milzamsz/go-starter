package observability

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// healthResponse is the JSON body returned by the health endpoint.
type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

// NewHealthHandler returns an http.HandlerFunc that checks database connectivity.
// It pings the provided pgxpool.Pool and returns:
//   - 200 with {"status":"ok","database":"up"} when the database is reachable
//   - 503 with {"status":"degraded","database":"down"} when the ping fails
func NewHealthHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		resp := healthResponse{
			Status:   "ok",
			Database: "up",
		}
		statusCode := http.StatusOK

		if err := pool.Ping(r.Context()); err != nil {
			resp.Status = "degraded"
			resp.Database = "down"
			statusCode = http.StatusServiceUnavailable
		}

		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(resp)
	}
}
