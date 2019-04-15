module github.com/torusresearch/torus-public

go 1.12

replace (
	github.com/tendermint/tendermint => github.com/YZhenY/tendermint v0.26.4-0.20190301084305-d515ebbc4bde
	github.com/testcontainers/testcontainer-go => github.com/testcontainers/testcontainers-go v0.0.0-20190108154635-47c0da630f72
)

require (
	github.com/Rican7/retry v0.1.0
	github.com/VividCortex/gohistogram v1.0.0 // indirect
	github.com/alecthomas/jsonschema v0.0.0-20190122210438-a6952de1bbe6 // indirect
	github.com/btcsuite/btcd v0.0.0-20190410025418-9bfb2ca0346b
	github.com/caarlos0/env v0.0.0-20190308142353-9a320ffe30f7
	github.com/ethereum/go-ethereum v1.8.26
	github.com/gogo/protobuf v1.2.1
	github.com/golang/protobuf v1.3.1
	github.com/google/go-cmp v0.2.0
	github.com/google/uuid v1.1.1
	github.com/intel-go/fastjson v0.0.0-20170329170629-f846ae58a1ab
	github.com/jackpal/gateway v1.0.5 // indirect
	github.com/jinzhu/copier v0.0.0-20180308034124-7e38e58719c3
	github.com/jmhodges/levigo v1.0.0 // indirect
	github.com/libp2p/go-libp2p v0.0.2
	github.com/libp2p/go-libp2p-crypto v0.0.1
	github.com/libp2p/go-libp2p-host v0.0.1
	github.com/libp2p/go-libp2p-net v0.0.1
	github.com/libp2p/go-libp2p-peer v0.0.1
	github.com/libp2p/go-libp2p-peerstore v0.0.1
	github.com/libp2p/go-libp2p-protocol v0.0.1
	github.com/looplab/fsm v0.1.0
	github.com/micro/go-config v1.1.0
	github.com/minio/sha256-simd v0.0.0-20190317211740-0d7ea3c244ee // indirect
	github.com/mr-tron/base58 v1.1.1 // indirect
	github.com/multiformats/go-multiaddr v0.0.2
	github.com/nbio/st v0.0.0-20140626010706-e9e8d9816f32 // indirect
	github.com/osamingo/jsonrpc v0.0.0-20171209064156-92d51847b98f
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v0.9.3-0.20190127221311-3c4408c8b829
	github.com/rs/cors v1.6.0
	github.com/streadway/quantile v0.0.0-20150917103942-b0c588724d25 // indirect
	github.com/stretchr/testify v1.3.0
	github.com/tendermint/btcd v0.1.1
	github.com/tendermint/go-amino v0.14.1 // indirect
	github.com/tendermint/tendermint v0.29.1
	github.com/tidwall/gjson v1.2.1
	github.com/tidwall/match v1.0.1 // indirect
	github.com/tidwall/pretty v0.0.0-20180105212114-65a9db5fad51 // indirect
	github.com/torusresearch/bijson v0.0.0-20190412111544-04b14d25087c
	github.com/torusresearch/torus-tester v0.0.0-20190415184417-d478fdd4004f // indirect
	github.com/tsenart/vegeta v6.3.0+incompatible
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.1.0 // indirect
	github.com/xlab-si/emmy v0.0.0-20181126075652-2a3c9dd29847
	github.com/ybbus/jsonrpc v2.1.2+incompatible
	google.golang.org/appengine v1.5.0 // indirect
	gopkg.in/h2non/baloo.v3 v3.0.2
	gopkg.in/h2non/gentleman.v2 v2.0.3 // indirect
)
