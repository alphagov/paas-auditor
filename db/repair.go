package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	cf "github.com/alphagov/paas-usage-events-collector/cloudfoundry"
)

// RepairEvents populates START events for applications and services that are
// missing due to the events being emitted before data collection began (T0).
//
// * If an application guid is currently in a running state according to
//   CloudController, but no events exist, then a START event is created for T0
// * If a service_instance_guid is currently in a CREATED state according to
//   CloudController, but no events exist, then a CREATED event is created for T0
func (pc *PostgresClient) RepairEvents(cfClient cf.Client) (err error) {
	tx, txErr := pc.Conn.Begin()
	if txErr != nil {
		return txErr
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	// remove any events with id=0
	_, err = tx.Exec(`delete from app_usage_events where id = 0`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`delete from service_usage_events where id = 0`)
	if err != nil {
		return err
	}

	// fetch the most recent event timestamp
	var firstEventTimestamp *time.Time
	err = tx.QueryRow(`
		select least((
			select min(created_at::timestamptz) from app_usage_events where created_at is not null
		),(
			select min(created_at::timestamptz) from service_usage_events where created_at is not null
		));
	`).Scan(&firstEventTimestamp)
	if err != nil {
		return err
	}
	if firstEventTimestamp == nil {
		return errors.New("Database appears to be empty and thus cannot be repaired.")
	}

	// fetch list of app guids we have events for
	rows, err := tx.Query(`
		select distinct
			raw_message->>'app_guid'
		from
			app_usage_events
		where
			raw_message->>'app_guid' is not null
	union
		select distinct
			raw_message->>'service_instance_guid'
		from
			service_usage_events
		where
			raw_message->>'service_instance_guid' is not null
	`)
	if err != nil {
		return err
	}
	knownGuids := map[string]bool{}
	for rows.Next() {
		var guid string
		if err := rows.Scan(&guid); err != nil {
			return err
		}
		knownGuids[guid] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// cache all the things
	orgs, err := cfClient.GetOrgs()
	if err != nil {
		return err
	}
	spaces, err := cfClient.GetSpaces()
	if err != nil {
		return err
	}
	services, err := cfClient.GetServices()
	if err != nil {
		return err
	}
	servicePlans, err := cfClient.GetServicePlans()
	if err != nil {
		return err
	}

	// fetch all the running apps
	apps, err := cfClient.GetApps()
	if err != nil {
		return err
	}
	for _, app := range apps {
		// Skip non running apps (FIXME: ListApps() could probably filter this)
		if app.State != StateStarted {
			continue
		}
		// Skip anything updated since we started collecting events
		updated, err := time.Parse(time.RFC3339, app.UpdatedAt)
		if err != nil {
			return err
		}
		fmt.Println("app", app.Guid)
		// Skip if we already have events for this app
		if seen := knownGuids[app.Guid]; seen {
			continue
		}
		fmt.Println("detected missing app", app.Guid)
		if updated.After(*firstEventTimestamp) {
			fmt.Println("not seen events for app", app.Guid, "but skipping since it was updated", updated, "which is after we started collecting data")
			continue
		}
		space, ok := spaces[app.SpaceGuid]
		if !ok {
			return errors.New("failed to find space: " + app.SpaceGuid)
		}
		// We have found a running app that we have no events for
		ev := map[string]interface{}{
			"state":                              "STARTED",
			"app_guid":                           app.Guid,
			"app_name":                           app.Name,
			"org_guid":                           space.OrganizationGuid,
			"task_guid":                          nil,
			"task_name":                          nil,
			"space_guid":                         space.Guid,
			"space_name":                         space.Name,
			"process_type":                       "web",
			"package_state":                      "STAGED",
			"buildpack_guid":                     nil,
			"buildpack_name":                     nil,
			"instance_count":                     app.Instances,
			"previous_state":                     "STOPPED",
			"parent_app_guid":                    app.Guid,
			"parent_app_name":                    app.Name,
			"previous_package_state":             "UNKNOWN",
			"previous_instance_count":            0,
			"memory_in_mb_per_instance":          app.Memory,
			"previous_memory_in_mb_per_instance": app.Memory,
		}
		fmt.Println("adding missing app STARTED event", ev)
		evJSON, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`
			insert into app_usage_events (
				id
				guid,
				created_at,
				raw_message
			) values (
				0,
				$1::text,
				$2::timestamp,
				$3::jsonb
			)
		`, app.Guid, firstEventTimestamp, string(evJSON))
		if err != nil {
			return err
		}
		knownGuids[app.Guid] = true
	}

	srvs, err := cfClient.GetServiceInstances()
	if err != nil {
		return err
	}
	for _, serviceInstance := range srvs {
		// Skip anything updated since we started collecting events
		updated, err := time.Parse(time.RFC3339, serviceInstance.UpdatedAt)
		if err != nil {
			return err
		}
		if updated.After(*firstEventTimestamp) {
			continue
		}
		// Skip if we already have events for this service instance
		if seen := knownGuids[serviceInstance.Guid]; seen {
			continue
		}
		space, ok := spaces[serviceInstance.SpaceGuid]
		if !ok {
			return errors.New("failed to find space: " + serviceInstance.SpaceGuid)
		}
		org, ok := orgs[space.OrganizationGuid]
		if !ok {
			return errors.New("failed to find org: " + space.OrganizationGuid)
		}
		servicePlan, ok := servicePlans[serviceInstance.ServicePlanGuid]
		if !ok {
			return errors.New("failed to find service plan for:" + serviceInstance.ServicePlanGuid)
		}
		service, ok := services[serviceInstance.ServiceGuid]
		if !ok {
			return errors.New("failed to find service for plan:" + serviceInstance.ServiceGuid)
		}

		// We have found a running service instance that we have no events for
		ev := map[string]interface{}{
			"state":                 "CREATED",
			"org_guid":              org.Guid,
			"space_guid":            space.Guid,
			"space_name":            space.Name,
			"service_guid":          service.Guid,
			"service_label":         service.Label,
			"service_plan_guid":     servicePlan.Guid,
			"service_plan_name":     servicePlan.Name,
			"service_instance_guid": serviceInstance.Guid,
			"service_instance_name": serviceInstance.Name,
			"service_instance_type": "managed_service_instance",
		}
		evJSON, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		fmt.Println("adding missing app CREATED event", ev)
		_, err = tx.Exec(`
			insert into service_usage_events (
				id
				guid,
				created_at,
				raw_message
			) values (
				0,
				$1::text,
				$2::timestamp,
				$3::jsonb
			)
		`, serviceInstance.Guid, firstEventTimestamp, string(evJSON))
		if err != nil {
			return err
		}
		knownGuids[serviceInstance.Guid] = true
	}

	return nil
}
