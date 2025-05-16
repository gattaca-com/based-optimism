package core

import "github.com/ethereum-optimism/optimism/op-service/eth"

type AltSync interface {
	// RequestL2Range informs the sync source that the given range of L2 blocks is missing,
	// and should be retrieved from any available alternative syncing source.
	// The start and end of the range are exclusive:
	// the start is the head we already have, the end is the first thing we have queued up.
	// It's the task of the alt-sync mechanism to use this hint to fetch the right payloads.
	// Note that the end and start may not be consistent: in this case the sync method should fetch older history
	//
	// If the end value is zeroed, then the sync-method may determine the end free of choice,
	// e.g. sync till the chain head meets the wallclock time. This functionality is optional:
	// a fixed target to sync towards may be determined by picking up payloads through P2P gossip or other sources.
	//
	// The sync results should be returned back to the driver via the OnUnsafeL2Payload(ctx, payload) method.
	// The latest requested range should always take priority over previous requests.
	// There may be overlaps in requested ranges.
	// An error may be returned if the scheduling fails immediately, e.g. a context timeout.
	RequestL2Range(ctx context.Context, start, end eth.L2BlockRef) error
}

type CLSync interface {
	LowestQueuedUnsafeBlock() eth.L2BlockRef
}

type AltSyncDriver struct {
}

// TODO altSyncTicker
// Alt-sync LowestQueuedUnsafeBlock should just be tried on some pace
//  -> request sync -> engine returns sync-from -> alt-sync does the rest with UnsafeL2Head
//  -> hold off on trigger if already deriving actively

//// If the engine is not ready, or if the L2 head is actively changing, then reset the alt-sync:
//// there is no need to request L2 blocks when we are syncing already.
//if head := s.Engine.UnsafeL2Head(); head != lastUnsafeL2 || !s.Derivation.DerivationReady() {
//lastUnsafeL2 = head
//altSyncTicker.Reset(syncCheckInterval)
//}

//// Create a ticker to check if there is a gap in the engine queue. Whenever
//// there is, we send requests to sync source to retrieve the missing payloads.
//syncCheckInterval := time.Duration(s.Config.BlockTime) * time.Second * 2
//altSyncTicker := time.NewTicker(syncCheckInterval)
//defer altSyncTicker.Stop()
//lastUnsafeL2 := s.Engine.UnsafeL2Head()

// 			// Check if there is a gap in the current unsafe payload queue.
//			ctx, cancel := context.WithTimeout(s.driverCtx, time.Second*2)
//			err := s.checkForGapInUnsafeQueue(ctx)
//			cancel()
//			if err != nil {
//				s.log.Warn("failed to check for unsafe L2 blocks to sync", "err", err)
//			}

// checkForGapInUnsafeQueue checks if there is a gap in the unsafe queue and attempts to retrieve the missing payloads from an alt-sync method.
// WARNING: This is only an outgoing signal, the blocks are not guaranteed to be retrieved.
// Results are received through OnUnsafeL2Payload.
func checkForGapInUnsafeQueue(ctx context.Context) error {
	start := s.Engine.UnsafeL2Head()
	end := s.CLSync.LowestQueuedUnsafeBlock()
	// Check if we have missing blocks between the start and end. Request them if we do.
	if end == (eth.L2BlockRef{}) {
		s.log.Debug("requesting sync with open-end range", "start", start)
		return s.altSync.RequestL2Range(ctx, start, eth.L2BlockRef{})
	} else if end.Number > start.Number+1 {
		s.log.Debug("requesting missing unsafe L2 block range", "start", start, "end", end, "size", end.Number-start.Number)
		return s.altSync.RequestL2Range(ctx, start, end)
	}
	return nil
}
