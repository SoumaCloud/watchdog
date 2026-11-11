package investigation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/julienschmidt/httprouter"
	"watchdog.onebusaway.org/internal/db/dbtest"
)

// newClusterSchemesRouter returns a router serving getClusterScheme for GET
// and HEAD, mounted without the request pipeline (see newServersRouter for why).
func newClusterSchemesRouter(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()
	s, _ := newTestService(t)
	s.Pool = pool

	router := httprouter.New()
	router.GET("/v1/cluster-schemes/:scheme_version", s.getClusterScheme)
	router.HEAD("/v1/cluster-schemes/:scheme_version", s.getClusterScheme)
	return router
}

// insertClusterScheme records a cluster scheme and returns its scheme_version.
func insertClusterScheme(t *testing.T, pool *pgxpool.Pool, version string, s2Level int, stationGrouping, description string) string {
	t.Helper()
	if err := dbtest.Exec(pool,
		"INSERT INTO cluster_scheme (scheme_version, s2_level, station_grouping, description) VALUES ($1, $2, $3, $4)",
		version, s2Level, stationGrouping, description); err != nil {
		t.Fatalf("insert cluster_scheme %q: %v", version, err)
	}
	return version
}

func TestGetClusterScheme_Registered(t *testing.T) {
	_, router := newTestService(t)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		if h, _, _ := router.Lookup(method, "/v1/cluster-schemes/:scheme_version"); h == nil {
			t.Errorf("%s /v1/cluster-schemes/:scheme_version is not registered", method)
		}
	}
}

func TestGetClusterScheme_RejectsUnknownParameter(t *testing.T) {
	wantInvalidParam(t, doGet(t, newClusterSchemesRouter(t, nil), "/v1/cluster-schemes/v1?foo=bar", nil), "foo")
}

func TestGetClusterScheme_UnknownVersion(t *testing.T) {
	pool := dbtest.NewMigratedPool(t)
	wantNotFound(t, doGet(t, newClusterSchemesRouter(t, pool), "/v1/cluster-schemes/nonexistent", nil))
}

func TestGetClusterScheme_ReturnsRepresentation(t *testing.T) {
	pool := dbtest.NewMigratedPool(t)
	if err := dbtest.Exec(pool,
		"INSERT INTO cluster_scheme (scheme_version, s2_level, station_grouping, description) VALUES ('v1', 14, 'parent_station', 'Level-14 S2 cells')"); err != nil {
		t.Fatalf("insert cluster_scheme: %v", err)
	}

	rec := doGet(t, newClusterSchemesRouter(t, pool), "/v1/cluster-schemes/v1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	var body clusterScheme
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.SchemeVersion != "v1" {
		t.Errorf("scheme_version = %q, want \"v1\"", body.SchemeVersion)
	}
	if body.S2Level != 14 {
		t.Errorf("s2_level = %d, want 14", body.S2Level)
	}
	if body.StationGrouping != "parent_station" {
		t.Errorf("station_grouping = %q, want \"parent_station\"", body.StationGrouping)
	}
	if body.Links["self"] != "/v1/cluster-schemes/v1" {
		t.Errorf("links.self = %q, want /v1/cluster-schemes/v1", body.Links["self"])
	}
}

func TestGetClusterScheme_Head(t *testing.T) {
	pool := dbtest.NewMigratedPool(t)
	if err := dbtest.Exec(pool,
		"INSERT INTO cluster_scheme (scheme_version, s2_level, station_grouping, description) VALUES ('v1', 14, 'parent_station', 'Level-14 S2 cells')"); err != nil {
		t.Fatalf("insert cluster_scheme: %v", err)
	}

	path := "/v1/cluster-schemes/v1"
	router := newClusterSchemesRouter(t, pool)
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
