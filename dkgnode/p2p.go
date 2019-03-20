package dkgnode

import (
	"context"
	"fmt"
	"log"

	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	ma "github.com/multiformats/go-multiaddr"
)

type P2PSuite struct {
	Host        host.Host
	HostAddress ma.Multiaddr
}

// SetupP2PHost creates a LibP2P host with an ID being the supplied private key and initiates
// the required suite
func SetupP2PHost(suite *Suite) (host.Host, error) {

	// Set keypair to node private key
	priv, _, err := crypto.ECDSAKeyPairFromKey(suite.EthSuite.NodePrivateKey)
	if err != nil {
		panic(err)
	}

	//TODO: Configure security right
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(suite.Config.P2PListenAddress),
		libp2p.Identity(priv),
		libp2p.DisableRelay(),
	}

	h, err := libp2p.New(context.Background(), opts...)
	if err != nil {
		return nil, err
	}

	// Build host multiaddress
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", h.ID().Pretty()))

	// Now we can build a full multiaddress to reach this host
	// by encapsulating both addresses:
	addr := h.Addrs()[0]
	fullAddr := addr.Encapsulate(hostAddr)
	log.Printf("P2P Full Address: %s\n", fullAddr)

	suite.P2PSuite.Host = h
	suite.P2PSuite.HostAddress = fullAddr

	return h, nil
}
