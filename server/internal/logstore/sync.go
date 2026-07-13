package logstore

import (
	"context"
	"log"
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
	track := newBackfillTracker()

	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	s.sync(ctx, source(), track, results)

	for {
		select {
		case <-ctx.Done():
			return
		case res := <-results:
			delete(track.inFlight, res.key)
			switch {
			case res.excluded:
				track.done[res.key] = true
				log.Printf("logstore: %s/%s excluded from persistence: %v", res.key.host, short(res.key.id), res.err)
			case res.err != nil:
				// Only a failure costs an attempt; a success must not consume the
				// budget that later gap healing depends on.
				track.attempts[res.key]++
				if ctx.Err() == nil {
					log.Printf("logstore: backfill of %s/%s failed (attempt %d/%d): %v",
						res.key.host, short(res.key.id), track.attempts[res.key], maxBackfillAttempts, res.err)
				}
			default:
				track.complete(s, res.key)
			}
		case <-ticker.C:
			s.sync(ctx, source(), track, results)
		}
	}
}

// backfillTracker is the sync loop's per-generation bookkeeping: which
// generations have been read, which are being read, how many times reading them
// has failed, and which dropped-line gap the current read is healing.
type backfillTracker struct {
	attempts map[genKey]int
	done     map[genKey]bool
	inFlight map[genKey]bool
	healing  map[genKey]int64
}

func newBackfillTracker() *backfillTracker {
	return &backfillTracker{
		attempts: make(map[genKey]int),
		done:     make(map[genKey]bool),
		inFlight: make(map[genKey]bool),
		healing:  make(map[genKey]int64),
	}
}

// schedule reports whether a generation should be (re-)read now. A generation
// is read once; a fresh dropped-line gap makes it eligible again with a fresh
// retry budget, so a spent budget can never block gap healing.
func (t *backfillTracker) schedule(s *Store, key genKey, excluded bool) bool {
	if gap := s.gapAt(key); gap != 0 && gap != t.healing[key] {
		t.healing[key] = gap
		delete(t.done, key)
		delete(t.attempts, key)
	}
	if excluded || t.inFlight[key] || t.done[key] || t.attempts[key] >= maxBackfillAttempts {
		return false
	}
	t.inFlight[key] = true
	return true
}

// complete retires a generation that was read successfully, along with the
// startup resume snapshot and the gap the read healed.
func (t *backfillTracker) complete(s *Store, key genKey) {
	t.done[key] = true
	delete(t.attempts, key)
	s.clearResume(key)
	if healed := t.healing[key]; healed != 0 {
		s.clearGap(key, healed)
		delete(t.healing, key)
	}
}

// sync reconciles one container listing against the stored generations.
func (s *Store) sync(ctx context.Context, engine Engine, track *backfillTracker, results chan backfillResult) {
	snapshot, hostErrs, err := engine.ListContainersAllHosts(ctx)
	if err != nil {
		log.Printf("logstore: container listing failed: %v", err)
		return
	}
	failed := make(map[string]bool, len(hostErrs))
	for _, hostErr := range hostErrs {
		failed[hostErr.HostName] = true
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
			if !track.schedule(s, key, excluded) {
				continue
			}

			s.producers.Add(1)
			go s.backfill(ctx, engine, info, results)
		}
	}

	if err := s.markRemoved(ctx, failed, live, nowMS); err != nil {
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

// markRemoved stamps removed_ms on every stored generation the engine no longer
// reports: a container gone from a host that listed successfully, and every
// generation of a host that is no longer configured at all (it appears in
// neither the listing nor the host errors). A host that merely failed to list is
// left untouched — an unreachable host must not look like a mass container
// removal. The generation row and its lines stay either way: that is what keeps
// a rebuilt container's history readable under its name.
func (s *Store) markRemoved(ctx context.Context, failedHosts map[string]bool, live map[genKey]bool, nowMS int64) error {
	rows, err := s.db.QueryContext(ctx,
		"SELECT host, container_id FROM containers WHERE removed_ms IS NULL")
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
		if !live[key] && !failedHosts[key.host] {
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
