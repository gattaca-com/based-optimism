package node

import (
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	opmetrics "github.com/ethereum-optimism/optimism/op-service/metrics"
	oprpc "github.com/ethereum-optimism/optimism/op-service/rpc"
)

func newRPCServer(rpcCfg *RPCConfig, rollupCfg *rollup.Config, l2Client l2EthClient, dr driverClient,
	safeDB SafeDBReader, log log.Logger, metrics opmetrics.RPCMetricer, appVersion string) *oprpc.Server {
	server := oprpc.NewServer(rpcCfg.ListenAddr, rpcCfg.ListenPort, appVersion,
		oprpc.WithLogger(log),
		oprpc.WithCORSHosts([]string{"*"}), // CORS is not important on op-node, but we used to do this on the old op-node RPC server, so kept for compatibility.
		oprpc.WithRPCRecorder(metrics.NewRecorder("main")),
	)
	api := NewNodeAPI(rollupCfg, l2Client, dr, safeDB, log)
	server.AddAPI(rpc.API{
		Namespace: "optimism",
		Service:   api,
	})
	return server
}

func (s *rpcServer) EnableP2P(backend *p2p.APIBackend) {
	s.apis = append(s.apis, rpc.API{
		Namespace:     p2p.NamespaceRPC,
		Version:       "",
		Service:       backend,
		Authenticated: false,
	})
}

func (s *rpcServer) EnableBasedAPI(api *basedAPI) {
	s.apis = append(s.apis, rpc.API {
		Namespace:     "based",
		Version:       "",
		Service:       api,
		Authenticated: false,
	})
}

func (s *rpcServer) Start() error {
	srv := rpc.NewServer()
	if err := node.RegisterApis(s.apis, nil, srv); err != nil {
		return err
	}

	// The CORS and VHosts arguments below must be set in order for
	// other services to connect to the opnode. VHosts in particular
	// defaults to localhost, which will prevent containers from
	// calling into the opnode without an "invalid host" error.
	nodeHandler := node.NewHTTPHandlerStack(srv, []string{"*"}, []string{"*"}, nil)

	mux := http.NewServeMux()
	mux.Handle("/", nodeHandler)
	mux.HandleFunc("/healthz", healthzHandler(s.appVersion))

	hs, err := ophttp.StartHTTPServer(s.endpoint, mux)
	if err != nil {
		return fmt.Errorf("failed to start HTTP RPC server: %w", err)
	}
	s.httpServer = hs
	return nil
}

func (r *rpcServer) Stop(ctx context.Context) error {
	return r.httpServer.Stop(ctx)
}

func (r *rpcServer) Addr() net.Addr {
	return r.httpServer.Addr()
}

func healthzHandler(appVersion string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(appVersion))
	}
}
