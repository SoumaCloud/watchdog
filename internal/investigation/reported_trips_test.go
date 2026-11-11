package investigation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/julienschmidt/httprouter"
	"watchdog.onebusaway.org/internal/db/dbtest"
)

// newReportedTripsRouter returns a router serving getReportedTrip for GET and
// HEAD, with the agency loader registered so ?include=agency works.
//
// Like newServersRouter, it mounts the handlers without the request pipeline.
func newReportedTripsRouter(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()
	s, _ := newTestService(t)
	s.Pool = pool

	router := httprouter.New()
	router.GET("/v1/reported-trips/:reported_trip_id", s.getReportedTrip)
	router.HEAD("/v1/reported-trips/:reported_trip_id", s.getReportedTrip)
	router.GET("/v1/agencies/:agency_id", s.getAgency)
	return router
}

// insertReportedTrip records a reported trip and returns its ID.
func insertReportedTrip(t *testing.T, pool *pgxpool.Pool, agencyID int64, obaTripID string) int64 {
	t.Helper()
	return dbtest.ID(t, pool,
		"INSERT INTO reported_trip (agency_id, oba_trip_id) VALUES ($1, $2) RETURNING reported_trip_id",
		agencyID, obaTripID)
}

func TestGetReportedTrip_Registered(t *testing.T) {
	_, router := newTestService(t)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		if h, _, _ := router.Lookup(method, "/v1/reported-trips/:reported_trip_id"); h == nil {
			t.Errorf("%s /v1/reported-trips/:reported_trip_id is not registered", method)
		}
	}
}

func TestGetReportedTrip_RejectsUnknownParameter(t *testing.T) {
	wantInvalidParam(t, doGet(t, newReportedTripsRouter(t, nil), "/v1/reported-trips/1?foo=bar", nil), "foo")
}

func TestGetReportedTrip_RejectsBadInclude(t *testing.T) {
	for _, query := range []string{"include=self", "include=server", "include=", "include=agency,agency"} {
		t.Run(query, func(t *testing.T) {
			wantInvalidParam(t, doGet(t, newReportedTripsRouter(t, nil), "/v1/reported-trips/1?"+query, nil), "include")
		})
	}
}

func TestGetReportedTrip_MalformedIDReturns404(t *testing.T) {
	router := newReportedTripsRouter(t, nil)
	for _, id := range []string{"abc", "0", "-1", "07"} {
		t.Run(id, func(t *testing.T) {
			wantNotFound(t, doGet(t, router, "/v1/reported-trips/"+id, nil))
		})
	}
}

func TestGetReportedTrip_UnknownID(t *testing.T) {
	pool := dbtest.NewMigratedPool(t)
	wantNotFound(t, doGet(t, newReportedTripsRouter(t, pool), "/v1/reported-trips/999999", nil))
}

func TestGetReportedTrip_ReturnsRepresentation(t *testing.T) {
	pool := dbtest.NewMigratedPool(t)
	if err := dbtest.Exec(pool,
		"INSERT INTO onebusaway_server (server_id, normalized_base_url, display_name) OVERRIDING SYSTEM VALUE VALUES (7, 'https://example.com', 'Example')"); err != nil {
		t.Fatalf("insert server: %v", err)
	}
	if err := dbtest.Exec(pool,
		"INSERT INTO agency (agency_id, server_id, external_agency_id) OVERRIDING SYSTEM VALUE VALUES (40, 7, '1')"); err != nil {
		t.Fatalf("insert agency: %v", err)
	}
	if err := dbtest.Exec(pool,
		"INSERT INTO reported_trip (reported_trip_id, agency_id, oba_trip_id) OVERRIDING SYSTEM VALUE VALUES (200, 40, 'TRIP-200')"); err != nil {
		t.Fatalf("insert reported_trip: %v", err)
	}

	rec := doGet(t, newReportedTripsRouter(t, pool), "/v1/reported-trips/200", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	want := `{"reported_trip_id":200,"agency_id":40,"oba_trip_id":"TRIP-200",` +
		`"links":{"agency":"/v1/agencies/40","self":"/v1/reported-trips/200"}}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n%s\nwant\n%s", got, want)
	}
}

func TestGetReportedTrip_IncludeAgency(t *testing.T) {
	pool := dbtest.NewMigratedPool(t)
	if err := dbtest.Exec(pool,
		"INSERT INTO onebusaway_server (server_id, normalized_base_url, display_name) OVERRIDING SYSTEM VALUE VALUES (7, 'https://example.com', 'Example')"); err != nil {
		t.Fatalf("insert server: %v", err)
	}
	if err := dbtest.Exec(pool,
		"INSERT INTO agency (agency_id, server_id, external_agency_id) OVERRIDING SYSTEM VALUE VALUES (40, 7, '1')"); err != nil {
		t.Fatalf("insert agency: %v", err)
	}
	if err := dbtest.Exec(pool,
		"INSERT INTO reported_trip (reported_trip_id, agency_id, oba_trip_id) OVERRIDING SYSTEM VALUE VALUES (200, 40, 'TRIP-200')"); err != nil {
		t.Fatalf("insert reported_trip: %v", err)
	}

	rec := doGet(t, newReportedTripsRouter(t, pool), "/v1/reported-trips/200?include=agency", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	var body struct {
		Included map[string]json.RawMessage `json:"included"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	agencyKey := "/v1/agencies/40"
	if _, ok := body.Included[agencyKey]; !ok {
		t.Errorf("included[%q] missing; included = %v", agencyKey, body.Included)
	}
}

func TestGetReportedTrip_Head(t *testing.T) {
	pool := dbtest.NewMigratedPool(t)
	if err := dbtest.Exec(pool,
		"INSERT INTO onebusaway_server (server_id, normalized_base_url, display_name) OVERRIDING SYSTEM VALUE VALUES (7, 'https://example.com', 'Example')"); err != nil {
		t.Fatalf("insert server: %v", err)
	}
	if err := dbtest.Exec(pool,
		"INSERT INTO agency (agency_id, server_id, external_agency_id) OVERRIDING SYSTEM VALUE VALUES (40, 7, '1')"); err != nil {
		t.Fatalf("insert agency: %v", err)
	}
	if err := dbtest.Exec(pool,
		"INSERT INTO reported_trip (reported_trip_id, agency_id, oba_trip_id) OVERRIDING SYSTEM VALUE VALUES (200, 40, 'TRIP-200')"); err != nil {
		t.Fatalf("insert reported_trip: %v", err)
	}

	path := "/v1/reported-trips/200"
	router := newReportedTripsRouter(t, pool)
	get := doGet(t, router, path, nil)

	req := httptest.NewRequest(http.MethodHead, path, nil)
	head := httptest.NewRecorder()
	router.ServeHTTP(head, req)

	if head.Code != get.Code {
		t.Errorf("HEAD status = %d, GET status = %d", head.Code, get.Code)
	}
	if head.Body.Len() != 0 {
		t.Errorf("HEAD body is not empty")
	}
}

func reportedTripPath(id int64) string {
	return "/v1/reported-trips/" + strconv.FormatInt(id, 10)
}
