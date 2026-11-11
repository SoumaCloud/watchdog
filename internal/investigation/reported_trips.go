package investigation

import (
	"context"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"watchdog.onebusaway.org/internal/db/dbgen"
)

// kindReportedTrip is the include kind of a reported trip (E12).
const kindReportedTrip resourceKind = "reported_trip"

// reportedTrip is one reported trip in a response (E12).
type reportedTrip struct {
	ReportedTripID int64  `json:"reported_trip_id"`
	AgencyID       int64  `json:"agency_id"`
	ObaTripID      string `json:"oba_trip_id"`
	Links          links  `json:"links"`
}

// reportedTripIncludable is what include accepts on E12: a reported trip links
// to one agency.
var reportedTripIncludable = map[string]resourceKind{"agency": kindAgency}

// reportedTripBody is the E12 body: the reported trip, plus included when asked.
type reportedTripBody struct {
	reportedTrip
	Included included `json:"included,omitzero"`
}

// getReportedTrip serves GET /v1/reported-trips/:reported_trip_id (E12).
func (s *InvestigationService) getReportedTrip(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, ok := pathID(ps, "reported_trip_id")
	if !ok {
		writeNotFound(w)
		return
	}
	var bad invalidParams
	values := parseQuery(r, &bad, "include")
	inc := s.parseInclude(values, reportedTripIncludable, &bad)
	if len(bad) > 0 {
		writeInvalidParams(w, bad)
		return
	}

	var body reportedTripBody
	err := s.read(r.Context(), func(ctx context.Context, q *dbgen.Queries) error {
		row, err := q.GetReportedTrip(ctx, id)
		if err != nil {
			return err
		}
		l := reportedTripLinks(row)
		body.reportedTrip = reportedTripRepresentation(row, l)
		body.Included, err = s.loadIncluded(ctx, q, []itemLinks{l}, inc)
		return err
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, r, body, cacheShort())
}

// reportedTripLinks is everything one reported trip links to.
func reportedTripLinks(row dbgen.ReportedTrip) itemLinks {
	return itemLinks{
		"self": {
			Kind: kindReportedTrip,
			ID:   row.ReportedTripID,
			URL:  "/v1/reported-trips/" + strconv.FormatInt(row.ReportedTripID, 10),
		},
		"agency": {
			Kind: kindAgency,
			ID:   row.AgencyID,
			URL:  "/v1/agencies/" + strconv.FormatInt(row.AgencyID, 10),
		},
	}
}

// reportedTripRepresentation maps one row to its E12 representation.
func reportedTripRepresentation(row dbgen.ReportedTrip, l itemLinks) reportedTrip {
	return reportedTrip{
		ReportedTripID: row.ReportedTripID,
		AgencyID:       row.AgencyID,
		ObaTripID:      row.ObaTripID,
		Links:          l.links(),
	}
}

// loadReportedTrips is the include loader for kindReportedTrip.
func loadReportedTrips(ctx context.Context, q *dbgen.Queries, ids []int64) (map[int64]any, error) {
	rows, err := q.ListReportedTripsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]any, len(rows))
	for _, row := range rows {
		out[row.ReportedTripID] = reportedTripRepresentation(row, reportedTripLinks(row))
	}
	return out, nil
}
