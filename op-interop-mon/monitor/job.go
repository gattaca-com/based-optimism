package monitor

import (
	"errors"
	"fmt"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/processors"
	supervisortypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var ErrNotExecutingMessage = errors.New("not an executing message")

type jobStatus int

const (
	jobStatusUnknown jobStatus = iota
	jobStatusFuture
	jobStatusValid
	jobStatusInvalid
	jobStatusMissing
)

func (s jobStatus) String() string {
	switch s {
	case jobStatusUnknown:
		return "unknown"
	case jobStatusFuture:
		return "future"
	case jobStatusValid:
		return "valid"
	case jobStatusInvalid:
		return "invalid"
	case jobStatusMissing:
		return "missing"
	default:
		return fmt.Sprintf("unknown status: %d", s)
	}
}

type Job struct {
	firstSeen     time.Time
	lastEvaluated time.Time

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
	if len(j.status) == 0 {
		j.status = append(j.status, status)
		return
	}
	if j.LatestStatus() != status {
		j.status = append(j.status, status)
		return
	}
}

func (j *Job) LatestStatus() jobStatus {
	if len(j.status) == 0 {
		return jobStatusUnknown
	}
	return j.status[len(j.status)-1]
}

func JobFromExecutingMessageLog(log *types.Log) (Job, error) {
	msg, err := processors.MessageFromLog(log)
	if err != nil {
		return Job{}, err
	}
	if msg == nil {
		return Job{}, ErrNotExecutingMessage
	}
	fmt.Println("msg", msg)
	return Job{
		executingAddress: log.Address,
		executingChain:   eth.ChainID(msg.Identifier.ChainID),
		executingBlock:   eth.BlockID{Hash: log.BlockHash, Number: log.BlockNumber},
		executingPayload: msg.PayloadHash,

		initiating: &msg.Identifier,
	}, nil
}

// BlockReceiptsToJobs converts a slice of receipts to a slice of jobs
func BlockReceiptsToJobs(receipts []*types.Receipt) []Job {
	jobs := make([]Job, 0, len(receipts))
	for _, receipt := range receipts {
		for _, log := range receipt.Logs {
			job, err := JobFromExecutingMessageLog(log)
			if err != nil {
				continue
			}
			jobs = append(jobs, job)
		}
	}
	return jobs
}

func (j *Job) UpdateLastEvaluated(t time.Time) {
	j.lastEvaluated = t
}

func (j *Job) String() string {
	return fmt.Sprintf("Job{executing: %s@%d:%s, payload: %s, initiating: %s@%d:%d, status: %v}",
		j.executingChain,
		j.executingBlock.Number,
		j.executingBlock.Hash.String()[:10],
		j.executingPayload.String()[:10],
		j.initiating.ChainID,
		j.initiating.BlockNumber,
		j.initiating.LogIndex,
		j.LatestStatus().String())
}
