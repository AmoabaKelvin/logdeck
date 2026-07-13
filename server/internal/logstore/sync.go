package logstore

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// syncLoop tracks container lifecycle by polling ListContainersAllHosts. It
// records generation metadata (name, image, compose project), stamps
// removed_ms on generations the engine no longer knows about, and schedules a
// backfill for every generation it has not read yet.
//
// Polling rather than a second docker.StreamEngineEvents subscription: the
// store needs the listing anyway (image, creation time, and the authoritative
// "this generation is gone" signal that no single event guarantees after a
// missed reconnect), and an events stream would only shave seconds off a path
// where the hub is already tailing live lines. This keeps one engine event
// stream in the process — the hub's.
func (s *Store) syncLoop(ctx context.Context, source func() Engine) {
	results := make(chan backfillResult, maxConcurrentBackfills)
	attempts := make(map[genKey]int)
	inFlight := make(map[genKey]bool)

	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	s.sync(ctx, source(), attempts, inFlight, results)

	for {
		select {
		case <-ctx.Done():
			return
		case res := <-results:
			delete(inFlight, res.key)
			switch {
			case res.excluded:
				log.Printf("logstore: %s/%s excluded from persistence: %v", res.key.host, short(res.key.id), res.err)
			case res.err != nil && ctx.Err() == nil:
				log.Printf("logstore: backfill of %s/%s failed (attempt %d/%d): %v",
					res.key.host, short(res.key.id), attempts[res.key], maxBackfillAttempts, res.err)
			}
		case <-ticker.C:
			s.sync(ctx, source(), attempts, inFlight, results)
		}
	}
}

// sync reconciles one container listing against the stored generations.
func (s *Store) sync(ctx context.Context, engine Engine, attempts map[genKey]int, inFlight map[genKey]bool, results chan backfillResult) {
	snapshot, hostErrs, err := engine.ListContainersAllHosts(ctx)
	if err != nil {
		log.Printf("logstore: container listing failed: %v", err)
		return
	}
	for _, hostErr := range hostErrs {
		log.Printf("logstore: listing containers on host %s failed: %v", hostErr.HostName, hostErr.Err)
	}

	nowMS := time.Now().UnixMilli()
	live := make(map[genKey]bool)

	for host, containers := range snapshot {
		for _, info := range containers {
			key := genKey{host: host, id: info.ID}
			live[key] = true

			excluded, err := s.upsertMeta(ctx, key, info, nowMS)
			if err != nil {
				log.Printf("logstore: recording container %s/%s failed: %v", host, short(info.ID), err)
				continue
			}
			if excluded || inFlight[key] || attempts[key] >= maxBackfillAttempts {
				continue
			}

			attempts[key]++
			inFlight[key] = true
			s.producers.Add(1)
			go s.backfill(ctx, engine, info, results)
		}
	}

	// Hosts that failed to list keep their generations untouched: an
	// unreachable host must not look like a mass container removal.
	listed := make([]string, 0, len(snapshot))
	for host := range snapshot {
		listed = append(listed, host)
	}
	if err := s.markRemoved(ctx, listed, live, nowMS); err != nil {
		log.Printf("logstore: marking removed containers failed: %v", err)
	}
}

// upsertMeta records the generation's engine metadata and reports whether the
// generation is excluded from persistence.
func (s *Store) upsertMeta(ctx context.Context, key genKey, info models.ContainerInfo, nowMS int64) (bool, error) {
	firstSeenMS := nowMS
	if info.Created > 0 {
		firstSeenMS = info.Created * 1000
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO containers (host, container_id, name, compose_project, image, first_seen_ms, last_seen_ms, removed_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL)
		ON CONFLICT(host, container_id) DO UPDATE SET
			name = excluded.name,
			compose_project = excluded.compose_project,
			image = excluded.image,
			first_seen_ms = min(containers.first_seen_ms, excluded.first_seen_ms),
			last_seen_ms = excluded.last_seen_ms,
			removed_ms = NULL`,
		key.host, key.id, containerName(info), composeProject(info.Labels), info.Image, firstSeenMS, nowMS,
	); err != nil {
		return false, err
	}

	var reason string
	err := s.db.QueryRowContext(ctx,
		"SELECT excluded_reason FROM containers WHERE host = ? AND container_id = ?",
		key.host, key.id,
	).Scan(&reason)
	return reason != "", err
}

// markRemoved stamps removed_ms on every stored generation of a listed host
// that the engine no longer reports. The generation row and its lines stay:
// that is what keeps a rebuilt container's history readable under its name.
func (s *Store) markRemoved(ctx context.Context, listedHosts []string, live map[genKey]bool, nowMS int64) error {
	if len(listedHosts) == 0 {
		return nil
	}

	args := make([]any, 0, len(listedHosts))
	for _, host := range listedHosts {
		args = append(args, host)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(listedHosts)), ",")

	rows, err := s.db.QueryContext(ctx,
		"SELECT host, container_id FROM containers WHERE removed_ms IS NULL AND host IN ("+placeholders+")",
		args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	var gone []genKey
	for rows.Next() {
		var key genKey
		if err := rows.Scan(&key.host, &key.id); err != nil {
			return err
		}
		if !live[key] {
			gone = append(gone, key)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, key := range gone {
		if _, err := s.db.ExecContext(ctx,
			"UPDATE containers SET removed_ms = ? WHERE host = ? AND container_id = ? AND removed_ms IS NULL",
			nowMS, key.host, key.id); err != nil {
			return err
		}
	}
	return nil
}

// short abbreviates an engine container ID for log lines.
func short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
