package alerts

import (
	"sync"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

// history is a bounded in-memory store of fired alerts, newest first. The
// alert history territory fills in the implementation.
type history struct {
	mu      sync.Mutex
	max     int
	entries []models.Alert
}
