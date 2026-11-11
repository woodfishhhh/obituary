package core

import (
	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

type Identity = corefs.Identity

type Item = ledger.Item

type Burial = ledger.Burial

type PurgeIntent = ledger.PurgeIntent

type BurialParent = ledger.BurialParent

type BuriedChild = ledger.BuriedChild

type Stats = ledger.Stats

type Candidate = ledger.Candidate

const ProposalContainer = ledger.ProposalContainer
const ProposalOnboard = ledger.ProposalOnboard
const ProposalLarge = ledger.ProposalLarge
const ProposalLeftovers = ledger.ProposalLeftovers

type Proposal = ledger.Proposal

const ConsentApproved = ledger.ConsentApproved
const ConsentDeclined = ledger.ConsentDeclined

type ProjectState = ledger.ProjectState

type LeftoverRejection = ledger.LeftoverRejection

type State = ledger.State

func ProjectKey(root string) string { return ledger.ProjectKey(root) }
