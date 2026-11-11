package ledger

import (
	"time"

	"github.com/woodfishhhh/obituary/internal/docker"
)

// ContainerIntent survives a canceled lifespan so an interrupted stop can be
// reconciled with the user's latest decision. Each intent reserves completion and
// ambiguous-action audit slots so diagnostics survive a full outbox.
type ContainerIntent struct {
	Token        string     `json:"token"`
	Operation    string     `json:"operation"`
	At           time.Time  `json:"at"`
	Item         *Item      `json:"item,omitempty"`
	Burial       *Burial    `json:"burial,omitempty"`
	Phase        string     `json:"phase,omitempty"`
	WasRunning   *bool      `json:"wasRunning,omitempty"`
	StoppedByUs  bool       `json:"stoppedByUs,omitempty"`
	StopTimeout  *int       `json:"stopTimeout,omitempty"`
	Diagnostic   string     `json:"diagnostic,omitempty"`
	RunningSince *time.Time `json:"runningSince,omitempty"`
	RemovalIDs   []string   `json:"removalIds,omitempty"`
}

func ContainerIntentIdentity(p ContainerIntent) *docker.Identity {
	if p.Item != nil {
		return p.Item.Container
	}
	if p.Burial != nil {
		return p.Burial.Container
	}
	return nil
}

func ContainerIntentID(p ContainerIntent) int64 {
	if p.Item != nil {
		return p.Item.ID
	}
	if p.Burial != nil {
		return p.Burial.ID
	}
	return 0
}

func FindContainerIntent(st *State, id int64) *ContainerIntent {
	for i := range st.ContainerIntents {
		if ContainerIntentID(st.ContainerIntents[i]) == id {
			return &st.ContainerIntents[i]
		}
	}
	return nil
}

func validateContainerIntents(st State) error {
	seen := map[int64]bool{}
	for _, p := range st.ContainerIntents {
		id := ContainerIntentIdentity(p)
		if p.Token == "" || p.At.IsZero() || ContainerIntentID(p) <= 0 || seen[ContainerIntentID(p)] || id == nil || !id.Valid() {
			return docker.ErrIdentity
		}
		seen[ContainerIntentID(p)] = true
		switch p.Phase {
		case "", "prepared", "sent", "confirmed":
		default:
			return docker.ErrIdentity
		}
		if p.StoppedByUs && (p.WasRunning == nil || !*p.WasRunning) {
			return docker.ErrIdentity
		}
		switch p.Operation {
		case "stop", "resume":
			if p.Item == nil || p.Burial != nil || ContainerIdentity(p.Item.Kind, id) != nil {
				return docker.ErrIdentity
			}
		case "start", "rm":
			if p.Burial == nil || p.Item != nil || ContainerIdentity(p.Burial.Kind, id) != nil {
				return docker.ErrIdentity
			}
		default:
			return docker.ErrIdentity
		}
	}
	return nil
}
