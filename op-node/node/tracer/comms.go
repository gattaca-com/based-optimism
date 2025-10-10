package tracer

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type Tracer interface {
	OnNewL1Head(ctx context.Context, sig eth.L1BlockRef)
	OnUnsafeL2Payload(ctx context.Context, from peer.ID, payload *eth.ExecutionPayloadEnvelope)
	OnPublishL2Payload(ctx context.Context, payload *eth.ExecutionPayloadEnvelope)
	OnNewFrag(ctx context.Context, from peer.ID, frag *eth.SignedNewFrag)
	OnPublishNewFrag(ctx context.Context, from peer.ID, frag *eth.SignedNewFrag)
	OnSealFrag(ctx context.Context, from peer.ID, seal *eth.SignedSeal)
	OnPublishSealFrag(ctx context.Context, from peer.ID, seal *eth.SignedSeal)
	OnEnv(ctx context.Context, from peer.ID, env *eth.SignedEnv)
	OnPublishEnv(ctx context.Context, from peer.ID, env *eth.SignedEnv)
}
