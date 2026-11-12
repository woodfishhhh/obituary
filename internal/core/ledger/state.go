package ledger

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/docker"
)

// automatic reports whether an item was registered without an explicit
// request for that exact path, and so needs the Git re-check.
func (it Item) Automatic() bool {
	return it.Source == "watch" || it.Project != "" && automaticSource(it.Source)
}

func automaticSource(source string) bool {
	switch source {
	case "rule", "proposal", "session":
		return true
	}
	return false
}

type Identity = corefs.Identity

// Item is one path with a lifespan.
type Item struct {
	Kind      string           `json:"kind,omitempty"`
	Container *docker.Identity `json:"container,omitempty"`
	ID        int64            `json:"id"`
	// Rev changes on every modification; sweeps only apply results to the revision they processed.
	Rev         int64        `json:"rev"`
	Path        string       `json:"path"`
	IsDir       bool         `json:"isDir"`
	Identity    *Identity    `json:"identity,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	ExpiresAt   time.Time    `json:"expiresAt"`
	Force       bool         `json:"force,omitempty"`
	Note        string       `json:"note,omitempty"`
	Source      string       `json:"source,omitempty"`  // cli, rule, mktemp, session, ...
	Project     string       `json:"project,omitempty"` // Git work tree it belongs to
	Session     string       `json:"session,omitempty"` // agent session that produced it
	WatchID     int64        `json:"watchId,omitempty"` // folder policy that registered it
	Watch       *WatchTiming `json:"watch,omitempty"`
	Attempts    int          `json:"attempts,omitempty"`
	NextAttempt time.Time    `json:"nextAttempt,omitzero"`
	LastError   string       `json:"lastError,omitempty"`
}

// Due reports whether the item needs processing at now.
func (it Item) Due(now time.Time) bool {
	return !it.ExpiresAt.After(now) && !it.NextAttempt.After(now)
}

// Burial is an item lying in the graveyard: moved out of the way, still
// restorable until it is purged.
type Burial struct {
	Kind      string           `json:"kind,omitempty"`
	Container *docker.Identity `json:"container,omitempty"`
	ID        int64            `json:"id"` // same number the item had while tracked
	Path      string           `json:"path"`
	Grave     string           `json:"grave"` // where it lies now
	// GraveID pins the grave directory Obituary created, so a purge can tell
	// it apart from anything swapped in or redirected to its path later.
	GraveID          *Identity        `json:"graveId,omitempty"`
	Parents          []BurialParent   `json:"parents,omitempty"`
	LongNames        bool             `json:"longNames,omitempty"` // Path and Parents were expanded before burial
	IsDir            bool             `json:"isDir"`
	Identity         *Identity        `json:"identity,omitempty"`
	Size             int64            `json:"size"`
	BuriedAt         time.Time        `json:"buriedAt"`
	PurgeAt          time.Time        `json:"purgeAt"`
	Reason           string           `json:"reason"` // expired or bury
	Note             string           `json:"note,omitempty"`
	Source           string           `json:"source,omitempty"`
	Project          string           `json:"project,omitempty"`
	Session          string           `json:"session,omitempty"`
	WatchID          int64            `json:"watchId,omitempty"`
	Operation        *BurialOperation `json:"operation,omitempty"` // explicit deletion invocation, not the item's origin
	Contains         []int64          `json:"contains,omitempty"`  // retired lifespans moved with this directory
	Children         []BuriedChild    `json:"children,omitempty"`  // identities only; never authority to rebuild lifespans
	PurgeAttempts    int              `json:"purgeAttempts,omitempty"`
	NextPurgeAttempt time.Time        `json:"nextPurgeAttempt,omitzero"`
	LastPurgeError   string           `json:"lastPurgeError,omitempty"`
	PurgeIntent      *PurgeIntent     `json:"purgeIntent,omitempty"`
	IdentityMismatch string           `json:"identityMismatch,omitempty"` // retained after a failed move rollback; never purge
}

// PurgeIntent records authorization after verifyGrave, before deletion. The
// enclosing Burial retains the verified object and directory identities.
// It reserves one audit slot until deletion and accounting commit together.
type PurgeIntent struct {
	VerifiedAt time.Time `json:"verifiedAt"`
	Basis      string    `json:"basis"`
}

// BurialParent records the complete original ancestor chain, nearest first.
// Identity describes the resolved target; Reparse records the original link.
type BurialParent struct {
	Path     string   `json:"path"`
	Identity Identity `json:"identity"`
	Reparse  bool     `json:"reparse,omitempty"`
}

type BuriedChild struct {
	WatchID  int64     `json:"watchId,omitempty"`
	ID       int64     `json:"id"`
	Identity *Identity `json:"identity"`
}

// Old manifests stored full Items. Read only their identity fields, discarding
// every lifespan field; subsequent writes use the minimal representation.
func (c *BuriedChild) UnmarshalJSON(data []byte) error {
	type childIdentity BuriedChild
	return json.Unmarshal(data, (*childIdentity)(c))
}

// Stats are lifetime counters shown by `obit stats`.
type Stats struct {
	ContainersLost   int64 `json:"containersLost,omitempty"`
	DeleteBuried     int64 `json:"deleteBuried,omitempty"`
	Buried           int64 `json:"buried"`
	BuriedBytes      int64 `json:"buriedBytes"`
	Purged           int64 `json:"purged"`
	PurgedBytes      int64 `json:"purgedBytes"` // space actually reclaimed
	Resurrected      int64 `json:"resurrected"`
	ResurrectedBytes int64 `json:"resurrectedBytes"`
}

// Candidate is one path a proposal would give a lifespan.
type Candidate struct {
	Kind      string           `json:"kind,omitempty"`
	Container *docker.Identity `json:"container,omitempty"`
	Reason    string           `json:"reason,omitempty"`
	Rev       uint64           `json:"rev,omitempty"` // incremented whenever this path is proposed again
	Path      string           `json:"path"`
	Lifespan  string           `json:"lifespan"`
	Rule      string           `json:"rule,omitempty"` // empty for leftovers no rule covers
	Size      int64            `json:"size"`
}

// Proposal kinds.
const (
	ProposalContainer = "container" // containers created during a session
	ProposalOnboard   = "onboard"   // first automatic registrations in a project
	ProposalLarge     = "large"     // a registration above the size threshold
	ProposalLeftovers = "leftovers" // untracked items a session left without a lifespan
)

// Proposal is registrations waiting for the user's approval. Obituary never
// starts automatic work in a project, or registers something unusually big,
// without one being approved.
type Proposal struct {
	CandidateSeq uint64             `json:"candidateSeq,omitempty"` // never reused after pruning; legacy starts at max candidate Rev
	ID           int64              `json:"id"`
	Kind         string             `json:"kind"`
	Project      string             `json:"project"`
	Session      string             `json:"session,omitempty"`
	CreatedAt    time.Time          `json:"createdAt"`
	Candidates   []Candidate        `json:"candidates"`
	Notified     []string           `json:"notified,omitempty"` // sessions already told about it
	WatchID      int64              `json:"watchId,omitempty"`
	WatchReview  *WatchReview       `json:"watchReview,omitempty"`
	WatchBurial  *WatchBurialReview `json:"watchBurial,omitempty"`
}

// Size is the total size of the candidates.
func (p Proposal) Size() int64 {
	var n int64
	for _, c := range p.Candidates {
		n += c.Size
	}
	return n
}

// Consent states of a project.
const (
	ConsentApproved = "approved"
	ConsentDeclined = "declined"
)

// ProjectState records whether automatic registration may run in a project.
type ProjectState struct {
	Consent string    `json:"consent"`
	At      time.Time `json:"at"`
}

// LeftoverRejection keeps a session's declined paths across repeated finishes.
type LeftoverRejection struct {
	Project string    `json:"project"`
	Session string    `json:"session"`
	Paths   []string  `json:"paths"`
	At      time.Time `json:"at"` // last rejection; legacy records sort oldest
}

// State is the persisted ledger: tracked items, the graveyard, proposals,
// per-project consent and counters.
type State struct {
	Graveyard           *GraveyardReference `json:"graveyard,omitempty"`
	PreviousGraveyard   *GraveyardReference `json:"previousGraveyard,omitempty"`
	GraveyardIndex      *graveyardIndex     `json:"graveyardIndex,omitempty"`
	GraveyardGCCursor   string              `json:"graveyardGCCursor,omitempty"` // maintenance only; never deletion authority
	graveyardLoaded     bool
	graveyardOriginal   [32]byte
	DataID              string                  `json:"dataId,omitempty"` // paired with .obituary-data for explicit uninstall
	ContainerIntents    []ContainerIntent       `json:"containerIntents,omitempty"`
	PendingAudit        []Event                 `json:"pendingAudit,omitempty"`
	Version             int                     `json:"version"`
	NextID              int64                   `json:"nextId"`
	Items               []Item                  `json:"items"`
	Burials             []Burial                `json:"burials,omitempty"`
	RestoreCleanups     []RestoreCleanup        `json:"restoreCleanups,omitempty"`
	Proposals           []Proposal              `json:"proposals,omitempty"`
	Watches             []WatchPolicy           `json:"watches,omitempty"`
	WatchExemptions     []WatchExemption        `json:"watchExemptions,omitempty"`
	AutoExemptions      []AutoExemption         `json:"autoExemptions,omitempty"`
	ObservationReceipts map[string]time.Time    `json:"observationReceipts,omitempty"`
	Projects            map[string]ProjectState `json:"projects,omitempty"`
	LeftoverRejections  []LeftoverRejection     `json:"leftoverRejections,omitempty"`
	Stats               Stats                   `json:"stats"`
}

// ProjectKey normalises a project root for the Projects map.
func ProjectKey(root string) string { return strings.ToLower(filepath.Clean(root)) }

// ProposalIndex returns the position of the proposal with id, or -1.
func (st *State) ProposalIndex(id int64) int {
	for i, p := range st.Proposals {
		if p.ID == id {
			return i
		}
	}
	return -1
}
