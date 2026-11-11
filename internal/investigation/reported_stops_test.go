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

// newReportedStopsRouter returns a router serving getReportedStop for GET and
// HEAD, with the include loader registered so ?include=agency works.
//
// Like newServersRouter, it mounts the handlers without the request pipeline:
// until #197 replaces the authenticate stub, every request through s.get is
// rejected with 401.
func newReportedStopsRouter(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()
	s, _ := newTestService(t)
	s.Pool = pool

	router := httprouter.New()
	router.GET("/v1/reported-stops/:reported_stop_id", s.getReportedStop)
	router.HEAD("/v1/reported-stops/:reported_stop_id", s.getReportedStop)
	router.GET("/v1/agencies/:agency_id", s.getAgency)
	return router
}

// insertReportedStop records a reported stop and returns its ID.
func insertReportedStop(t *testing.T, pool *pgxpool.Pool, agencyID int64, obaStopID string) int64 {
	t.Helper()
	return dbtest.ID(t, pool,
		"INSERT INTO reported_stop (agency_id, oba_stop_id) VALUES ($1, $2) RETURNING reported_stop_id",
		agencyID, obaStopID)
}

func TestGetReportedStop_Registered(t *testing.T) {
	_, router := newTestService(t)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		if h, _, _ := router.Lookup(method, "/v1/reported-stops/:reported_stop_id"); h == nil {
			t.Errorf("%s /v1/reported-stops/:reported_stop_id is not registered", method)
		}
	}
}

func TestGetReportedStop_RejectsUnknownParameter(t *testing.T) {
	wantInvalidParam(t, doGet(t, newReportedStopsRouter(t, nil), "/v1/reported-stops/1?foo=bar", nil), "foo")
}

func TestGetReportedStop_RejectsBadInclude(t *testing.T) {
	for _, query := range []string{"include=self", "include=server", "include=", "include=agency,agency"} {
		t.Run(query, func(t *testing.T) {
			wantInvalidParam(t, doGet(t, newReportedStopsRouter(t, nil), "/v1/reported-stops/1?"+query, nil), "include")
		})
	}
}

func TestGetReportedStop_MalformedIDReturns404(t *testing.T) {
	router := newReportedStopsRouter(t, nil)
	for _, id := range []string{"abc", "0", "-1", "07"} {
		t.Run(id, func(t *testing.T) {
			wantNotFound(t, doGet(t, router, "/v1/reported-stops/"+id, nil))
		})
	}
}

func TestGetReportedStop_UnknownID(t *testing.T) {
	pool := dbtest.NewMigratedPool(t)
	wantNotFound(t, doGet(t, newReportedStopsRouter(t, pool), "/v1/reported-stops/999999", nil))
}

func TestGetReportedStop_ReturnsRepresentation(t *testing.T) {
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
		"INSERT INTO reported_stop (reported_stop_id, agency_id, oba_stop_id) OVERRIDING SYSTEM VALUE VALUES (100, 40, 'STOP-100')"); err != nil {
		t.Fatalf("insert reported_stop: %v", err)
	}

	rec := doGet(t, newReportedStopsRouter(t, pool), "/v1/reported-stops/100", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	want := `{"reported_stop_id":100,"agency_id":40,"oba_stop_id":"STOP-100",` +
		`"links":{"agency":"/v1/agencies/40","self":"/v1/reported-stops/100"}}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n%s\nwant\n%s", got, want)
	}
}

func TestGetReportedStop_IncludeAgency(t *testing.T) {
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
		"INSERT INTO reported_stop (reported_stop_id, agency_id, oba_stop_id) OVERRIDING SYSTEM VALUE VALUES (100, 40, 'STOP-100')"); err != nil {
		t.Fatalf("insert reported_stop: %v", err)
	}

	rec := doGet(t, newReportedStopsRouter(t, pool), "/v1/reported-stops/100?include=agency", nil)
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

func TestGetReportedStop_Head(t *testing.T) {
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
		"INSERT INTO reported_stop (reported_stop_id, agency_id, oba_stop_id) OVERRIDING SYSTEM VALUE VALUES (100, 40, 'STOP-100')"); err != nil {
		t.Fatalf("insert reported_stop: %v", err)
	}

	path := "/v1/reported-stops/100"
	router := newReportedStopsRouter(t, pool)
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
	if head.Header().Get("ETag") != get.Header().Get("ETag") {
		t.Errorf("HEAD ETag = %q, GET ETag = %q", head.Header().Get("ETag"), get.Header().Get("ETag"))
	}
}

func reportedStopPath(id int64) string {
	return "/v1/reported-stops/" + strconv.FormatInt(id, 10)
}
