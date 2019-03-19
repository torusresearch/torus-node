package dkgnode

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"

	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	ma "github.com/multiformats/go-multiaddr"
)

func setupP2PHost(suite *Suite) {
	// The context governs the lifetime of the libp2p node
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set keypair to node private key
	priv, _, err := crypto.ECDSAKeyPairFromKey(suite.EthSuite.NodePrivateKey)
	if err != nil {
		panic(err)
	}

	h2, err := libp2p.New(ctx,
		// Use your own created keypair
		libp2p.Identity(priv),

		// Set your own listen address
		// The config takes an array of addresses, specify as many as you want.
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/"+suite.Config.MyPort),
	)

	if err != nil {
		panic(err)
	}

	fmt.Printf("Hello World, my second hosts ID is %s\n", h2.ID())
}

// makeBasicHost creates a LibP2P host with a random peer ID listening on the
// given multiaddress. It won't encrypt the connection if insecure is true.
func makeBasicHost(listenPort int, insecure bool, privateKey *ecdsa.PrivateKey) (host.Host, error) {

	// Set keypair to node private key
	priv, _, err := crypto.ECDSAKeyPairFromKey(privateKey)
	if err != nil {
		panic(err)
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort)),
		libp2p.Identity(priv),
		libp2p.DisableRelay(),
	}

	if insecure {
		opts = append(opts, libp2p.NoSecurity)
	}

	basicHost, err := libp2p.New(context.Background(), opts...)
	if err != nil {
		return nil, err
	}

	// Build host multiaddress
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", basicHost.ID().Pretty()))

	// Now we can build a full multiaddress to reach this host
	// by encapsulating both addresses:
	addr := basicHost.Addrs()[0]
	fullAddr := addr.Encapsulate(hostAddr)
	log.Printf("I am %s\n", fullAddr)
	if insecure {
		log.Printf("Now run \"./echo -l %d -d %s -insecure\" on a different terminal\n", listenPort+1, fullAddr)
	} else {
		log.Printf("Now run \"./echo -l %d -d %s\" on a different terminal\n", listenPort+1, fullAddr)
	}

	return basicHost, nil
}
