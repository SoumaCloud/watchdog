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
	"watchdog.onebusaway.org/internal/models"
)

// newVehiclesRouter returns a router serving getVehicleEntity for GET and HEAD,
// with the endpoint loader registered so ?include=endpoint works.
//
// Like newServersRouter, it mounts the handlers without the request pipeline.
func newVehiclesRouter(t *testing.T, pool *pgxpool.Pool, servers ...models.ObaServer) http.Handler {
	t.Helper()
	s, _ := newTestService(t)
	s.Pool = pool
	s.Servers = func() []models.ObaServer { return servers }

	router := httprouter.New()
	router.GET("/v1/vehicles/:vehicle_entity_id", s.getVehicleEntity)
	router.HEAD("/v1/vehicles/:vehicle_entity_id", s.getVehicleEntity)
	router.GET("/v1/vehicle-position-endpoints/:endpoint_id", s.listServerVehiclePositionEndpoints)
	return router
}

// insertVehicleEntity records a vehicle entity and returns its ID.
func insertVehicleEntity(t *testing.T, pool *pgxpool.Pool, endpointID int64, externalID string) int64 {
	t.Helper()
	return dbtest.ID(t, pool,
		"INSERT INTO vehicle_entity (endpoint_id, external_vehicle_id) VALUES ($1, $2) RETURNING vehicle_entity_id",
		endpointID, externalID)
}

func TestGetVehicleEntity_Registered(t *testing.T) {
	_, router := newTestService(t)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		if h, _, _ := router.Lookup(method, "/v1/vehicles/:vehicle_entity_id"); h == nil {
			t.Errorf("%s /v1/vehicles/:vehicle_entity_id is not registered", method)
		}
	}
}

func TestGetVehicleEntity_RejectsUnknownParameter(t *testing.T) {
	wantInvalidParam(t, doGet(t, newVehiclesRouter(t, nil), "/v1/vehicles/1?foo=bar", nil), "foo")
}

func TestGetVehicleEntity_RejectsBadInclude(t *testing.T) {
	for _, query := range []string{"include=self", "include=server", "include=", "include=endpoint,endpoint"} {
		t.Run(query, func(t *testing.T) {
			wantInvalidParam(t, doGet(t, newVehiclesRouter(t, nil), "/v1/vehicles/1?"+query, nil), "include")
		})
	}
}

func TestGetVehicleEntity_MalformedIDReturns404(t *testing.T) {
	router := newVehiclesRouter(t, nil)
	for _, id := range []string{"abc", "0", "-1", "07"} {
		t.Run(id, func(t *testing.T) {
			wantNotFound(t, doGet(t, router, "/v1/vehicles/"+id, nil))
		})
	}
}

func TestGetVehicleEntity_UnknownID(t *testing.T) {
	pool := dbtest.NewMigratedPool(t)
	wantNotFound(t, doGet(t, newVehiclesRouter(t, pool), "/v1/vehicles/999999", nil))
}

func TestGetVehicleEntity_ReturnsRepresentation(t *testing.T) {
	pool := dbtest.NewMigratedPool(t)
	if err := dbtest.Exec(pool,
		"INSERT INTO onebusaway_server (server_id, normalized_base_url, display_name) OVERRIDING SYSTEM VALUE VALUES (7, 'https://example.com', 'Example')"); err != nil {
		t.Fatalf("insert server: %v", err)
	}
	if err := dbtest.Exec(pool,
		"INSERT INTO vehicle_position_endpoint (endpoint_id, server_id, normalized_vehicle_position_url) OVERRIDING SYSTEM VALUE VALUES (10, 7, 'https://example.com/vp')"); err != nil {
		t.Fatalf("insert endpoint: %v", err)
	}
	if err := dbtest.Exec(pool,
		"INSERT INTO vehicle_entity (vehicle_entity_id, endpoint_id, external_vehicle_id) OVERRIDING SYSTEM VALUE VALUES (500, 10, 'BUS-500')"); err != nil {
		t.Fatalf("insert vehicle_entity: %v", err)
	}

	rec := doGet(t, newVehiclesRouter(t, pool), "/v1/vehicles/500", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	var body vehicleEntity
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.VehicleEntityID != 500 {
		t.Errorf("vehicle_entity_id = %d, want 500", body.VehicleEntityID)
	}
	if body.EndpointID != 10 {
		t.Errorf("endpoint_id = %d, want 10", body.EndpointID)
	}
	if body.ExternalVehicleID != "BUS-500" {
		t.Errorf("external_vehicle_id = %q, want \"BUS-500\"", body.ExternalVehicleID)
	}
	if body.Links["self"] != "/v1/vehicles/500" {
		t.Errorf("links.self = %q", body.Links["self"])
	}
	if body.Links["vehicle_anomaly_episodes"] == "" {
		t.Error("links.vehicle_anomaly_episodes is missing")
	}
}

func TestGetVehicleEntity_Head(t *testing.T) {
	pool := dbtest.NewMigratedPool(t)
	if err := dbtest.Exec(pool,
		"INSERT INTO onebusaway_server (server_id, normalized_base_url, display_name) OVERRIDING SYSTEM VALUE VALUES (7, 'https://example.com', 'Example')"); err != nil {
		t.Fatalf("insert server: %v", err)
	}
	if err := dbtest.Exec(pool,
		"INSERT INTO vehicle_position_endpoint (endpoint_id, server_id, normalized_vehicle_position_url) OVERRIDING SYSTEM VALUE VALUES (10, 7, 'https://example.com/vp')"); err != nil {
		t.Fatalf("insert endpoint: %v", err)
	}
	if err := dbtest.Exec(pool,
		"INSERT INTO vehicle_entity (vehicle_entity_id, endpoint_id, external_vehicle_id) OVERRIDING SYSTEM VALUE VALUES (500, 10, 'BUS-500')"); err != nil {
		t.Fatalf("insert vehicle_entity: %v", err)
	}

	path := "/v1/vehicles/500"
	router := newVehiclesRouter(t, pool)
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

func vehiclePath(id int64) string {
	return "/v1/vehicles/" + strconv.FormatInt(id, 10)
}
