# hbbft
Practical implementation of the Honey Badger Byzantine Fault Tolerance consensus algorithm written in Go.

<p align="center">
  <a href="https://github.com/anthdm/hbbft/releases">
    <img src="https://img.shields.io/github/tag/anthdm/hbbft.svg?style=flat">
  </a>
  <a href="https://circleci.com/gh/anthdm/hbbft/tree/master">
    <img src="https://circleci.com/gh/anthdm/hbbft/tree/master.svg?style=shield">
  </a>
  <a href="https://goreportcard.com/report/github.com/anthdm/hbbft">
    <img src="https://goreportcard.com/badge/github.com/anthdm/hbbft">
  </a>
</p>

### Summary
This package includes the building blocks for implementing a practical version of the hbbft protocol. The exposed engine can be plugged easily into existing applications. Users can choose to use the transport layer or roll their own. For implementation details take a look at the simulations which implements hbbft into a realistic scenario. The building blocks of hbbft exist out of the following sub-protocols: 

#### Reliable broadcast (RBC)
Uses reedsolomon erasure encoding to disseminate an ecrypted set of transactions.

#### Binary Byzantine Agreement (BBA)
Uses a common coin to agree that a majority of the participants have a consensus that RBC has completed. 

#### Asynchronous Common Subset (ACS)
Combines RBC and BBA to agree on a set of encrypted transactions.

#### HoneyBadger
Top level HoneyBadger protocol that implements all the above sub(protocols) into a complete --production grade-- practical consensus engine. 

### Usage
Install dependencies
```
make deps
```

Run tests
```
make test
```

### How to plug hbbft in to your existing setup. 
Create a new instance of HoneyBadger.
```
// Create a Config struct with your prefered settings.
cfg := hbbft.Config{
    // The number of nodes in the network.
    N: 4,
    // Identifier of this node.
    ID: 101,
    // Identifiers of the participating nodes. 
    Nodes: uint64{67, 1, 99, 101},
    // The prefered batch size. If BatchSize is empty, an ideal batch size will
    // be choosen for you.
    BatchSize: 100,
}

// Create a new instance of the HoneyBadger engine and pass in the config.
hb := hbbft.NewHoneyBadger(cfg)
```

Filling the engine with transactions. Hbbft uses an interface to make it compatible with all types of transactions, the only contract a transaction have to fullfill is the `Hash() []byte` method.
```
// Transaction is an interface that abstract the underlying data of the actual
// transaction. This allows package hbbft to be easily adopted by other
// applications.
type Transaction interface {
	Hash() []byte
}

Adding new transactions can be done be calling the following method on the hb instance.
hb.AddTransaction(tx) // can be called in routines without any problem.
```

Starting the engine.
```
hb.Start() // will start proposing batches of transactions in the network. 
```

Applications build on top of hbbft can decide when they access commited transactions. Once consumed the output will be reset.
```
hb.Outputs() // returns a map of commited transactions per epoch.

for epoch, tx := range hb.Outputs() {
  fmt.Printf("batch for epoch %d: %v\n", epoch, tx)
}
```

>A working implementation can be found in the [bench folder](https://github.com/anthdm/hbbft/tree/master/bench), where hbbft is implemented over local transport.

### Current project state
- [x] Reliable Broadcast Algorithm
- [x] Binary Byzantine Agreement
- [x] Asynchronous Common Subset 
- [x] HoneyBadger top level protocol 

### TODO
- [ ] Treshold encryption
- [ ] Configurable serialization for transactions 

### References
- [The Honey Badger BFT protocols](https://eprint.iacr.org/2016/199.pdf)
- [Practical Byzantine Fault Tolerance](http://pmg.csail.mit.edu/papers/osdi99.pdf)
- [Treshold encryption](https://en.wikipedia.org/wiki/Threshold_cryptosystem)
- [Shared secret](https://en.wikipedia.org/wiki/Shared_secret)

### Other language implementations
- [Rust](https://github.com/poanetwork/hbbft)
- [Erlang](https://github.com/helium/erlang-hbbft)
