package investigation

import (
	"context"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"watchdog.onebusaway.org/internal/db/dbgen"
)

// clusterScheme is one cluster scheme in a response (E40).
type clusterScheme struct {
	SchemeVersion   string  `json:"scheme_version"`
	S2Level         int16   `json:"s2_level"`
	StationGrouping string  `json:"station_grouping"`
	Description     string  `json:"description"`
	CreatedAt       apiTime `json:"created_at"`
	Links           links   `json:"links"`
}

// getClusterScheme serves GET /v1/cluster-schemes/:scheme_version (E40).
// scheme_version is a TEXT primary key such as "v1", so this endpoint uses
// a plain string path parameter rather than an integer ID.
func (s *InvestigationService) getClusterScheme(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	version := ps.ByName("scheme_version")
	if version == "" {
		writeNotFound(w)
		return
	}
	var bad invalidParams
	parseQuery(r, &bad) // no query parameters accepted
	if len(bad) > 0 {
		writeInvalidParams(w, bad)
		return
	}

	var row dbgen.ClusterScheme
	err := s.read(r.Context(), func(ctx context.Context, q *dbgen.Queries) error {
		var err error
		row, err = q.GetClusterScheme(ctx, version)
		return err
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, r, clusterSchemeRepresentation(row), cacheShort())
}

// clusterSchemeRepresentation maps one row to its E40 representation.
func clusterSchemeRepresentation(row dbgen.ClusterScheme) clusterScheme {
	return clusterScheme{
		SchemeVersion:   row.SchemeVersion,
		S2Level:         row.S2Level,
		StationGrouping: row.StationGrouping,
		Description:     row.Description,
		CreatedAt:       newAPITime(row.CreatedAt),
		Links: links{
			"self": "/v1/cluster-schemes/" + row.SchemeVersion,
		},
	}
}
