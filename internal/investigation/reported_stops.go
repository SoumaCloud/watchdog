package investigation

import (
	"context"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"watchdog.onebusaway.org/internal/db/dbgen"
)

// kindReportedStop is the include kind of a reported stop (E09).
const kindReportedStop resourceKind = "reported_stop"

// reportedStop is one reported stop in a response (E09).
type reportedStop struct {
	ReportedStopID int64  `json:"reported_stop_id"`
	AgencyID       int64  `json:"agency_id"`
	ObaStopID      string `json:"oba_stop_id"`
	Links          links  `json:"links"`
}

// reportedStopIncludable is what include accepts on E09: a reported stop links
// to one agency.
var reportedStopIncludable = map[string]resourceKind{"agency": kindAgency}

// reportedStopBody is the E09 body: the reported stop, plus included when asked.
type reportedStopBody struct {
	reportedStop
	Included included `json:"included,omitzero"`
}

// getReportedStop serves GET /v1/reported-stops/:reported_stop_id (E09).
func (s *InvestigationService) getReportedStop(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, ok := pathID(ps, "reported_stop_id")
	if !ok {
		writeNotFound(w)
		return
	}
	var bad invalidParams
	values := parseQuery(r, &bad, "include")
	inc := s.parseInclude(values, reportedStopIncludable, &bad)
	if len(bad) > 0 {
		writeInvalidParams(w, bad)
		return
	}

	var body reportedStopBody
	err := s.read(r.Context(), func(ctx context.Context, q *dbgen.Queries) error {
		row, err := q.GetReportedStop(ctx, id)
		if err != nil {
			return err
		}
		l := reportedStopLinks(row)
		body.reportedStop = reportedStopRepresentation(row, l)
		body.Included, err = s.loadIncluded(ctx, q, []itemLinks{l}, inc)
		return err
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, r, body, cacheShort())
}

// reportedStopLinks is everything one reported stop links to.
func reportedStopLinks(row dbgen.ReportedStop) itemLinks {
	return itemLinks{
		"self": {
			Kind: kindReportedStop,
			ID:   row.ReportedStopID,
			URL:  "/v1/reported-stops/" + strconv.FormatInt(row.ReportedStopID, 10),
		},
		"agency": {
			Kind: kindAgency,
			ID:   row.AgencyID,
			URL:  "/v1/agencies/" + strconv.FormatInt(row.AgencyID, 10),
		},
	}
}

// reportedStopRepresentation maps one row to its E09 representation.
func reportedStopRepresentation(row dbgen.ReportedStop, l itemLinks) reportedStop {
	return reportedStop{
		ReportedStopID: row.ReportedStopID,
		AgencyID:       row.AgencyID,
		ObaStopID:      row.ObaStopID,
		Links:          l.links(),
	}
}

// loadReportedStops is the include loader for kindReportedStop.
func loadReportedStops(ctx context.Context, q *dbgen.Queries, ids []int64) (map[int64]any, error) {
	rows, err := q.ListReportedStopsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]any, len(rows))
	for _, row := range rows {
		out[row.ReportedStopID] = reportedStopRepresentation(row, reportedStopLinks(row))
	}
	return out, nil
}
