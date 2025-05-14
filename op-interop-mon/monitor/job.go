package monitor

import (
	"time"

	supervisortypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
)

type jobStatus int

const (
	jobStatusUnknown jobStatus = iota
	jobStatusFuture
	jobStatusValid
	jobStatusInvalid
	jobStatusMissing
)

type Job struct {
	firstSeen time.Time
	lastSeen  time.Time

	executingTxHash common.Hash
	executing       *supervisortypes.Identifier

	initiating *supervisortypes.Identifier

	// track each status seen over time
	status []jobStatus
}
