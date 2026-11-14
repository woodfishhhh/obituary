// Package core preserves the original API while its owners live in focused subpackages.
package core

import (
	"os"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

const ActionAbandoned = lifecycle.ActionAbandoned

const ActionAdded = lifecycle.ActionAdded

const ActionUpdated = lifecycle.ActionUpdated

const ActionExtended = lifecycle.ActionExtended

const ActionCanceled = lifecycle.ActionCanceled
const ActionRetried = lifecycle.ActionRetried

// RetryStopped marks an item whose burial stopped retrying (see Item.Stuck).
var RetryStopped = lifecycle.RetryStopped

const ActionRecycled = lifecycle.ActionRecycled

const ActionVanished = lifecycle.ActionVanished

const ActionReplaced = lifecycle.ActionReplaced

const ActionRefused = lifecycle.ActionRefused

const ActionRetry = lifecycle.ActionRetry

const ActionSkipped = lifecycle.ActionSkipped

type Event = lifecycle.Event

type AuditError = lifecycle.AuditError

func OnlyAuditErrors(err error) bool {
	return lifecycle.OnlyAuditErrors(err)
}

type AutoExemption = lifecycle.AutoExemption

type BurialOperation = lifecycle.BurialOperation

const KindContainer = lifecycle.KindContainer

type ContainerIntent = lifecycle.ContainerIntent

const MaxLifespan = lifecycle.MaxLifespan

func ParseDuration(text string) (time.Duration, error) {
	return lifecycle.ParseDuration(text)
}

func ParseAt(text string, now time.Time) (time.Time, error) {
	return lifecycle.ParseAt(text, now)
}

func FormatRemaining(left time.Duration) string {
	return lifecycle.FormatRemaining(left)
}

func BurialPathMatches(buried string, directory bool, path string) bool {
	return lifecycle.BurialPathMatches(buried, directory, path)
}

const ActionBuried = lifecycle.ActionBuried

const ActionResurrected = lifecycle.ActionResurrected

const ActionPurged = lifecycle.ActionPurged

const ActionLost = lifecycle.ActionLost

const ActionRecoveryPending = lifecycle.ActionRecoveryPending

const ActionNotExecuted = lifecycle.ActionNotExecuted

const ActionBlocked = lifecycle.ActionBlocked

const ReasonExpired = lifecycle.ReasonExpired

const ReasonBury = lifecycle.ReasonBury

type GraveResult = lifecycle.GraveResult

func GraveFailed(results []GraveResult) bool {
	return lifecycle.GraveFailed(results)
}

type GraveyardView = lifecycle.GraveyardView

func BurialReferenceID(ref string) (id int64, numeric bool, err error) {
	return lifecycle.BurialReferenceID(ref)
}

type PendingGrave = lifecycle.PendingGrave

func ReadMetadata(path string) ([]byte, error) {
	return lifecycle.ReadMetadata(path)
}

func WriteMetadata(path string, data []byte) error {
	return lifecycle.WriteMetadata(path, data)
}

type ObservationQueueStatus = lifecycle.ObservationQueueStatus

const ObservationMaxAge = lifecycle.ObservationMaxAge

const ObservationMaxAttempts = lifecycle.ObservationMaxAttempts

type Observation = lifecycle.Observation

var ErrProposalChanged = lifecycle.ErrProposalChanged

var ErrProposalBusy = lifecycle.ErrProposalBusy

type WatchReview = lifecycle.WatchReview

func ProposalFingerprint(p Proposal) string {
	return lifecycle.ProposalFingerprint(p)
}

func ValidateProposalReviews(refs, fingerprints []string) error {
	return lifecycle.ValidateProposalReviews(refs, fingerprints)
}

func ExistingProposals(proposals []Proposal) []Proposal {
	return lifecycle.ExistingProposals(proposals)
}

type ProposalResult = lifecycle.ProposalResult

type AddOptions = lifecycle.AddOptions

const MaxMetadataBytes = lifecycle.MaxMetadataBytes

func NormalizeSession(session string) string {
	return lifecycle.NormalizeSession(session)
}

func ValidateMetadata(field, value string) error {
	return lifecycle.ValidateMetadata(field, value)
}

type FileAncestry = lifecycle.FileAncestry

type RegistrationScope = lifecycle.RegistrationScope

type RestoreCleanup = lifecycle.RestoreCleanup

type Guard = lifecycle.Guard

type RefusedError = lifecycle.RefusedError

func DefaultGuard(home string, keep ...string) Guard {
	return lifecycle.DefaultGuard(home, keep...)
}

var ErrNotFound = lifecycle.ErrNotFound

var ErrUnsupported = lifecycle.ErrUnsupported

var ErrUnsafe = lifecycle.ErrUnsafe

var ErrProtected = lifecycle.ErrProtected

var ErrSweepBusy = lifecycle.ErrSweepBusy

type FS = lifecycle.FS

type RestoreParent = lifecycle.RestoreParent

const ModeGraveyard = lifecycle.ModeGraveyard

const ModeRecycle = lifecycle.ModeRecycle

const (
	ProposalsOff    = lifecycle.ProposalsOff
	ProposalsTiered = lifecycle.ProposalsTiered
	ProposalsAuto   = lifecycle.ProposalsAuto
	ProposalsAsk    = lifecycle.ProposalsAsk
)

type AutoReceipt = lifecycle.AutoReceipt

type AutoHandled = lifecycle.AutoHandled

const (
	AutoCreated     = lifecycle.AutoCreated
	AutoRegenerable = lifecycle.AutoRegenerable
	AutoContainer   = lifecycle.AutoContainer
	AutoAll         = lifecycle.AutoAll
)

const DefaultRetention = lifecycle.DefaultRetention

type Service = lifecycle.Service

type Ownership = lifecycle.Ownership

type OwnerBatchFunc = lifecycle.OwnerBatchFunc

type Result = lifecycle.Result

func Failed(results []Result) bool {
	return lifecycle.Failed(results)
}

type Identity = lifecycle.Identity

type Item = lifecycle.Item

type Burial = lifecycle.Burial

type PurgeIntent = lifecycle.PurgeIntent

type BurialParent = lifecycle.BurialParent

type BuriedChild = lifecycle.BuriedChild

type Stats = lifecycle.Stats

type Candidate = lifecycle.Candidate

const ProposalContainer = lifecycle.ProposalContainer

const ProposalOnboard = lifecycle.ProposalOnboard

const ProposalLarge = lifecycle.ProposalLarge

const ProposalLeftovers = lifecycle.ProposalLeftovers

type Proposal = lifecycle.Proposal

const ConsentApproved = lifecycle.ConsentApproved

const ConsentDeclined = lifecycle.ConsentDeclined

type ProjectState = lifecycle.ProjectState

type LeftoverRejection = lifecycle.LeftoverRejection

type State = lifecycle.State

func ProjectKey(root string) string {
	return lifecycle.ProjectKey(root)
}

type Summary = lifecycle.Summary

type Store = lifecycle.Store

const DataOwnerFile = lifecycle.DataOwnerFile

func VerifyDataOwner(root *os.Root) error {
	return lifecycle.VerifyDataOwner(root)
}

type GraveyardReference = lifecycle.GraveyardReference

func OwnedLedgerFile(root *os.Root, name string) bool {
	return lifecycle.OwnedLedgerFile(root, name)
}

type Outcome = lifecycle.Outcome

func SetFileSyncForTesting(fn func(*os.File) error) func() {
	return lifecycle.SetFileSyncForTesting(fn)
}

const WatchFirstSeen = lifecycle.WatchFirstSeen

const WatchModified = lifecycle.WatchModified

const ProposalWatch = lifecycle.ProposalWatch

type WatchPolicy = lifecycle.WatchPolicy

type WatchSkip = lifecycle.WatchSkip

func WatchDownloadSkips(path string) ([]WatchSkip, error) {
	return lifecycle.WatchDownloadSkips(path)
}

type WatchExclusion = lifecycle.WatchExclusion

type WatchExemption = lifecycle.WatchExemption

const WatchStableWindow = lifecycle.WatchStableWindow

type WatchSnapshot = lifecycle.WatchSnapshot

type WatchTiming = lifecycle.WatchTiming

const ProposalWatchBurial = lifecycle.ProposalWatchBurial

const ActionProposed = lifecycle.ActionProposed

type WatchBurialReview = lifecycle.WatchBurialReview

type WatchStatus = lifecycle.WatchStatus
