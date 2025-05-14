package monitor

import (
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/processors"
	supervisortypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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

	executingTx      common.Hash
	executingAddress common.Address
	executingChain   eth.ChainID
	executingBlock   eth.BlockID
	executingPayload common.Hash

	initiating *supervisortypes.Identifier

	// track each status seen over time
	status []jobStatus
}

func (j *Job) UpdateStatus(status jobStatus) {
	if j.LatestStatus() != status {
		j.status = append(j.status, status)
	}
}

func (j *Job) LatestStatus() jobStatus {
	return j.status[len(j.status)-1]
}

func JobFromExecutingMessageLog(log *types.Log) (Job, error) {
	msg, err := processors.MessageFromLog(log)
	if err != nil {
		return Job{}, err
	}
	return Job{
		executingAddress: log.Address,
		executingChain:   eth.ChainID(msg.Identifier.ChainID),
		executingBlock:   eth.BlockID{Hash: log.BlockHash, Number: log.BlockNumber},
		executingPayload: msg.PayloadHash,

		initiating: &msg.Identifier,
	}, nil
}
