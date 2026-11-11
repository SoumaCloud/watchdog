package investigation

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"watchdog.onebusaway.org/internal/db/dbgen"
)

// kindVehicleEntity is the include kind of a vehicle entity (E22).
const kindVehicleEntity resourceKind = "vehicle_entity"

// vehicleEntity is one vehicle entity in a response (E22).
type vehicleEntity struct {
	VehicleEntityID   int64  `json:"vehicle_entity_id"`
	EndpointID        int64  `json:"endpoint_id"`
	ExternalVehicleID string `json:"external_vehicle_id"`
	Links             links  `json:"links"`
}

// vehicleEntityIncludable is what include accepts on E22: a vehicle entity
// links to one endpoint.
var vehicleEntityIncludable = map[string]resourceKind{"endpoint": kindEndpoint}

// vehicleEntityBody is the E22 body: the vehicle entity, plus included when asked.
type vehicleEntityBody struct {
	vehicleEntity
	Included included `json:"included,omitzero"`
}

// getVehicleEntity serves GET /v1/vehicles/:vehicle_entity_id (E22).
func (s *InvestigationService) getVehicleEntity(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, ok := pathID(ps, "vehicle_entity_id")
	if !ok {
		writeNotFound(w)
		return
	}
	var bad invalidParams
	values := parseQuery(r, &bad, "include")
	inc := s.parseInclude(values, vehicleEntityIncludable, &bad)
	if len(bad) > 0 {
		writeInvalidParams(w, bad)
		return
	}

	var body vehicleEntityBody
	err := s.read(r.Context(), func(ctx context.Context, q *dbgen.Queries) error {
		row, err := q.GetVehicleEntity(ctx, id)
		if err != nil {
			return err
		}
		l := vehicleEntityLinks(row)
		body.vehicleEntity = vehicleEntityRepresentation(row, l)
		body.Included, err = s.loadIncluded(ctx, q, []itemLinks{l}, inc)
		return err
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.writeJSON(w, r, body, cacheShort())
}

// vehicleEntityLinks is everything one vehicle entity links to that can be
// included or is included elsewhere: itself and its endpoint.
func vehicleEntityLinks(row dbgen.VehicleEntity) itemLinks {
	return itemLinks{
		"self": {
			Kind: kindVehicleEntity,
			ID:   row.VehicleEntityID,
			URL:  "/v1/vehicles/" + strconv.FormatInt(row.VehicleEntityID, 10),
		},
		"endpoint": {
			Kind: kindEndpoint,
			ID:   row.EndpointID,
			URL:  "/v1/vehicle-position-endpoints/" + strconv.FormatInt(row.EndpointID, 10),
		},
	}
}

// vehicleEntityRepresentation maps one row to its E22 representation.
func vehicleEntityRepresentation(row dbgen.VehicleEntity, l itemLinks) vehicleEntity {
	out := l.links()
	out["vehicle_anomaly_episodes"] = linkWithQuery("/v1/vehicle-anomaly-episodes",
		url.Values{"vehicle_entity_id": {strconv.FormatInt(row.VehicleEntityID, 10)}})
	return vehicleEntity{
		VehicleEntityID:   row.VehicleEntityID,
		EndpointID:        row.EndpointID,
		ExternalVehicleID: row.ExternalVehicleID,
		Links:             out,
	}
}

// loadVehicleEntities is the include loader for kindVehicleEntity.
func loadVehicleEntities(ctx context.Context, q *dbgen.Queries, ids []int64) (map[int64]any, error) {
	rows, err := q.ListVehicleEntitiesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]any, len(rows))
	for _, row := range rows {
		out[row.VehicleEntityID] = vehicleEntityRepresentation(row, vehicleEntityLinks(row))
	}
	return out, nil
}
