// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package nodelist

import (
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = abi.U256
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// NodelistABI is the input ABI used to generate the binding from.
const NodelistABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"nodeList\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"epoch\",\"type\":\"uint256\"}],\"name\":\"viewNodeListCount\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"epoch\",\"type\":\"uint256\"}],\"name\":\"viewNodes\",\"outputs\":[{\"name\":\"\",\"type\":\"address[]\"},{\"name\":\"\",\"type\":\"uint256[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"},{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"addressToNodeDetailsLog\",\"outputs\":[{\"name\":\"declaredIp\",\"type\":\"string\"},{\"name\":\"position\",\"type\":\"uint256\"},{\"name\":\"pubKx\",\"type\":\"uint256\"},{\"name\":\"pubKy\",\"type\":\"uint256\"},{\"name\":\"tmP2PListenAddress\",\"type\":\"string\"},{\"name\":\"p2pListenAddress\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"epoch\",\"type\":\"uint256\"},{\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"viewWhitelist\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"epoch\",\"type\":\"uint256\"},{\"name\":\"nodeAddress\",\"type\":\"address\"},{\"name\":\"allowed\",\"type\":\"bool\"}],\"name\":\"updateWhitelist\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"isOwner\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"epoch\",\"type\":\"uint256\"},{\"name\":\"node\",\"type\":\"address\"}],\"name\":\"viewNodeDetails\",\"outputs\":[{\"name\":\"declaredIp\",\"type\":\"string\"},{\"name\":\"position\",\"type\":\"uint256\"},{\"name\":\"tmP2PListenAddress\",\"type\":\"string\"},{\"name\":\"p2pListenAddress\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"epoch\",\"type\":\"uint256\"},{\"name\":\"declaredIp\",\"type\":\"string\"},{\"name\":\"pubKx\",\"type\":\"uint256\"},{\"name\":\"pubKy\",\"type\":\"uint256\"},{\"name\":\"tmP2PListenAddress\",\"type\":\"string\"},{\"name\":\"p2pListenAddress\",\"type\":\"string\"}],\"name\":\"listNode\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"viewLatestEpoch\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"publicKey\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"epoch\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"position\",\"type\":\"uint256\"}],\"name\":\"NodeListed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"}]"

// Nodelist is an auto generated Go binding around an Ethereum contract.
type Nodelist struct {
	NodelistCaller     // Read-only binding to the contract
	NodelistTransactor // Write-only binding to the contract
	NodelistFilterer   // Log filterer for contract events
}

// NodelistCaller is an auto generated read-only Go binding around an Ethereum contract.
type NodelistCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NodelistTransactor is an auto generated write-only Go binding around an Ethereum contract.
type NodelistTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NodelistFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type NodelistFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NodelistSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type NodelistSession struct {
	Contract     *Nodelist         // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// NodelistCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type NodelistCallerSession struct {
	Contract *NodelistCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts   // Call options to use throughout this session
}

// NodelistTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type NodelistTransactorSession struct {
	Contract     *NodelistTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// NodelistRaw is an auto generated low-level Go binding around an Ethereum contract.
type NodelistRaw struct {
	Contract *Nodelist // Generic contract binding to access the raw methods on
}

// NodelistCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type NodelistCallerRaw struct {
	Contract *NodelistCaller // Generic read-only contract binding to access the raw methods on
}

// NodelistTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type NodelistTransactorRaw struct {
	Contract *NodelistTransactor // Generic write-only contract binding to access the raw methods on
}

// NewNodelist creates a new instance of Nodelist, bound to a specific deployed contract.
func NewNodelist(address common.Address, backend bind.ContractBackend) (*Nodelist, error) {
	contract, err := bindNodelist(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Nodelist{NodelistCaller: NodelistCaller{contract: contract}, NodelistTransactor: NodelistTransactor{contract: contract}, NodelistFilterer: NodelistFilterer{contract: contract}}, nil
}

// NewNodelistCaller creates a new read-only instance of Nodelist, bound to a specific deployed contract.
func NewNodelistCaller(address common.Address, caller bind.ContractCaller) (*NodelistCaller, error) {
	contract, err := bindNodelist(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &NodelistCaller{contract: contract}, nil
}

// NewNodelistTransactor creates a new write-only instance of Nodelist, bound to a specific deployed contract.
func NewNodelistTransactor(address common.Address, transactor bind.ContractTransactor) (*NodelistTransactor, error) {
	contract, err := bindNodelist(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &NodelistTransactor{contract: contract}, nil
}

// NewNodelistFilterer creates a new log filterer instance of Nodelist, bound to a specific deployed contract.
func NewNodelistFilterer(address common.Address, filterer bind.ContractFilterer) (*NodelistFilterer, error) {
	contract, err := bindNodelist(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &NodelistFilterer{contract: contract}, nil
}

// bindNodelist binds a generic wrapper to an already deployed contract.
func bindNodelist(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(NodelistABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Nodelist *NodelistRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Nodelist.Contract.NodelistCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Nodelist *NodelistRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Nodelist.Contract.NodelistTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Nodelist *NodelistRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Nodelist.Contract.NodelistTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Nodelist *NodelistCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Nodelist.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Nodelist *NodelistTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Nodelist.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Nodelist *NodelistTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Nodelist.Contract.contract.Transact(opts, method, params...)
}

// AddressToNodeDetailsLog is a free data retrieval call binding the contract method 0x1bcd979d.
//
// Solidity: function addressToNodeDetailsLog(address , uint256 ) constant returns(string declaredIp, uint256 position, uint256 pubKx, uint256 pubKy, string tmP2PListenAddress, string p2pListenAddress)
func (_Nodelist *NodelistCaller) AddressToNodeDetailsLog(opts *bind.CallOpts, arg0 common.Address, arg1 *big.Int) (struct {
	DeclaredIp         string
	Position           *big.Int
	PubKx              *big.Int
	PubKy              *big.Int
	TmP2PListenAddress string
	P2pListenAddress   string
}, error) {
	ret := new(struct {
		DeclaredIp         string
		Position           *big.Int
		PubKx              *big.Int
		PubKy              *big.Int
		TmP2PListenAddress string
		P2pListenAddress   string
	})
	out := ret
	err := _Nodelist.contract.Call(opts, out, "addressToNodeDetailsLog", arg0, arg1)
	return *ret, err
}

// AddressToNodeDetailsLog is a free data retrieval call binding the contract method 0x1bcd979d.
//
// Solidity: function addressToNodeDetailsLog(address , uint256 ) constant returns(string declaredIp, uint256 position, uint256 pubKx, uint256 pubKy, string tmP2PListenAddress, string p2pListenAddress)
func (_Nodelist *NodelistSession) AddressToNodeDetailsLog(arg0 common.Address, arg1 *big.Int) (struct {
	DeclaredIp         string
	Position           *big.Int
	PubKx              *big.Int
	PubKy              *big.Int
	TmP2PListenAddress string
	P2pListenAddress   string
}, error) {
	return _Nodelist.Contract.AddressToNodeDetailsLog(&_Nodelist.CallOpts, arg0, arg1)
}

// AddressToNodeDetailsLog is a free data retrieval call binding the contract method 0x1bcd979d.
//
// Solidity: function addressToNodeDetailsLog(address , uint256 ) constant returns(string declaredIp, uint256 position, uint256 pubKx, uint256 pubKy, string tmP2PListenAddress, string p2pListenAddress)
func (_Nodelist *NodelistCallerSession) AddressToNodeDetailsLog(arg0 common.Address, arg1 *big.Int) (struct {
	DeclaredIp         string
	Position           *big.Int
	PubKx              *big.Int
	PubKy              *big.Int
	TmP2PListenAddress string
	P2pListenAddress   string
}, error) {
	return _Nodelist.Contract.AddressToNodeDetailsLog(&_Nodelist.CallOpts, arg0, arg1)
}

// IsOwner is a free data retrieval call binding the contract method 0x8f32d59b.
//
// Solidity: function isOwner() constant returns(bool)
func (_Nodelist *NodelistCaller) IsOwner(opts *bind.CallOpts) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _Nodelist.contract.Call(opts, out, "isOwner")
	return *ret0, err
}

// IsOwner is a free data retrieval call binding the contract method 0x8f32d59b.
//
// Solidity: function isOwner() constant returns(bool)
func (_Nodelist *NodelistSession) IsOwner() (bool, error) {
	return _Nodelist.Contract.IsOwner(&_Nodelist.CallOpts)
}

// IsOwner is a free data retrieval call binding the contract method 0x8f32d59b.
//
// Solidity: function isOwner() constant returns(bool)
func (_Nodelist *NodelistCallerSession) IsOwner() (bool, error) {
	return _Nodelist.Contract.IsOwner(&_Nodelist.CallOpts)
}

// NodeList is a free data retrieval call binding the contract method 0x02098741.
//
// Solidity: function nodeList(uint256 , uint256 ) constant returns(address)
func (_Nodelist *NodelistCaller) NodeList(opts *bind.CallOpts, arg0 *big.Int, arg1 *big.Int) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _Nodelist.contract.Call(opts, out, "nodeList", arg0, arg1)
	return *ret0, err
}

// NodeList is a free data retrieval call binding the contract method 0x02098741.
//
// Solidity: function nodeList(uint256 , uint256 ) constant returns(address)
func (_Nodelist *NodelistSession) NodeList(arg0 *big.Int, arg1 *big.Int) (common.Address, error) {
	return _Nodelist.Contract.NodeList(&_Nodelist.CallOpts, arg0, arg1)
}

// NodeList is a free data retrieval call binding the contract method 0x02098741.
//
// Solidity: function nodeList(uint256 , uint256 ) constant returns(address)
func (_Nodelist *NodelistCallerSession) NodeList(arg0 *big.Int, arg1 *big.Int) (common.Address, error) {
	return _Nodelist.Contract.NodeList(&_Nodelist.CallOpts, arg0, arg1)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_Nodelist *NodelistCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _Nodelist.contract.Call(opts, out, "owner")
	return *ret0, err
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_Nodelist *NodelistSession) Owner() (common.Address, error) {
	return _Nodelist.Contract.Owner(&_Nodelist.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_Nodelist *NodelistCallerSession) Owner() (common.Address, error) {
	return _Nodelist.Contract.Owner(&_Nodelist.CallOpts)
}

// ViewLatestEpoch is a free data retrieval call binding the contract method 0xe439c099.
//
// Solidity: function viewLatestEpoch() constant returns(uint256)
func (_Nodelist *NodelistCaller) ViewLatestEpoch(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Nodelist.contract.Call(opts, out, "viewLatestEpoch")
	return *ret0, err
}

// ViewLatestEpoch is a free data retrieval call binding the contract method 0xe439c099.
//
// Solidity: function viewLatestEpoch() constant returns(uint256)
func (_Nodelist *NodelistSession) ViewLatestEpoch() (*big.Int, error) {
	return _Nodelist.Contract.ViewLatestEpoch(&_Nodelist.CallOpts)
}

// ViewLatestEpoch is a free data retrieval call binding the contract method 0xe439c099.
//
// Solidity: function viewLatestEpoch() constant returns(uint256)
func (_Nodelist *NodelistCallerSession) ViewLatestEpoch() (*big.Int, error) {
	return _Nodelist.Contract.ViewLatestEpoch(&_Nodelist.CallOpts)
}

// ViewNodeDetails is a free data retrieval call binding the contract method 0xa93a0fb0.
//
// Solidity: function viewNodeDetails(uint256 epoch, address node) constant returns(string declaredIp, uint256 position, string tmP2PListenAddress, string p2pListenAddress)
func (_Nodelist *NodelistCaller) ViewNodeDetails(opts *bind.CallOpts, epoch *big.Int, node common.Address) (struct {
	DeclaredIp         string
	Position           *big.Int
	TmP2PListenAddress string
	P2pListenAddress   string
}, error) {
	ret := new(struct {
		DeclaredIp         string
		Position           *big.Int
		TmP2PListenAddress string
		P2pListenAddress   string
	})
	out := ret
	err := _Nodelist.contract.Call(opts, out, "viewNodeDetails", epoch, node)
	return *ret, err
}

// ViewNodeDetails is a free data retrieval call binding the contract method 0xa93a0fb0.
//
// Solidity: function viewNodeDetails(uint256 epoch, address node) constant returns(string declaredIp, uint256 position, string tmP2PListenAddress, string p2pListenAddress)
func (_Nodelist *NodelistSession) ViewNodeDetails(epoch *big.Int, node common.Address) (struct {
	DeclaredIp         string
	Position           *big.Int
	TmP2PListenAddress string
	P2pListenAddress   string
}, error) {
	return _Nodelist.Contract.ViewNodeDetails(&_Nodelist.CallOpts, epoch, node)
}

// ViewNodeDetails is a free data retrieval call binding the contract method 0xa93a0fb0.
//
// Solidity: function viewNodeDetails(uint256 epoch, address node) constant returns(string declaredIp, uint256 position, string tmP2PListenAddress, string p2pListenAddress)
func (_Nodelist *NodelistCallerSession) ViewNodeDetails(epoch *big.Int, node common.Address) (struct {
	DeclaredIp         string
	Position           *big.Int
	TmP2PListenAddress string
	P2pListenAddress   string
}, error) {
	return _Nodelist.Contract.ViewNodeDetails(&_Nodelist.CallOpts, epoch, node)
}

// ViewNodeListCount is a free data retrieval call binding the contract method 0x0bf1a62e.
//
// Solidity: function viewNodeListCount(uint256 epoch) constant returns(uint256)
func (_Nodelist *NodelistCaller) ViewNodeListCount(opts *bind.CallOpts, epoch *big.Int) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Nodelist.contract.Call(opts, out, "viewNodeListCount", epoch)
	return *ret0, err
}

// ViewNodeListCount is a free data retrieval call binding the contract method 0x0bf1a62e.
//
// Solidity: function viewNodeListCount(uint256 epoch) constant returns(uint256)
func (_Nodelist *NodelistSession) ViewNodeListCount(epoch *big.Int) (*big.Int, error) {
	return _Nodelist.Contract.ViewNodeListCount(&_Nodelist.CallOpts, epoch)
}

// ViewNodeListCount is a free data retrieval call binding the contract method 0x0bf1a62e.
//
// Solidity: function viewNodeListCount(uint256 epoch) constant returns(uint256)
func (_Nodelist *NodelistCallerSession) ViewNodeListCount(epoch *big.Int) (*big.Int, error) {
	return _Nodelist.Contract.ViewNodeListCount(&_Nodelist.CallOpts, epoch)
}

// ViewNodes is a free data retrieval call binding the contract method 0x13a7cd36.
//
// Solidity: function viewNodes(uint256 epoch) constant returns(address[], uint256[])
func (_Nodelist *NodelistCaller) ViewNodes(opts *bind.CallOpts, epoch *big.Int) ([]common.Address, []*big.Int, error) {
	var (
		ret0 = new([]common.Address)
		ret1 = new([]*big.Int)
	)
	out := &[]interface{}{
		ret0,
		ret1,
	}
	err := _Nodelist.contract.Call(opts, out, "viewNodes", epoch)
	return *ret0, *ret1, err
}

// ViewNodes is a free data retrieval call binding the contract method 0x13a7cd36.
//
// Solidity: function viewNodes(uint256 epoch) constant returns(address[], uint256[])
func (_Nodelist *NodelistSession) ViewNodes(epoch *big.Int) ([]common.Address, []*big.Int, error) {
	return _Nodelist.Contract.ViewNodes(&_Nodelist.CallOpts, epoch)
}

// ViewNodes is a free data retrieval call binding the contract method 0x13a7cd36.
//
// Solidity: function viewNodes(uint256 epoch) constant returns(address[], uint256[])
func (_Nodelist *NodelistCallerSession) ViewNodes(epoch *big.Int) ([]common.Address, []*big.Int, error) {
	return _Nodelist.Contract.ViewNodes(&_Nodelist.CallOpts, epoch)
}

// ViewWhitelist is a free data retrieval call binding the contract method 0x22d1b9db.
//
// Solidity: function viewWhitelist(uint256 epoch, address nodeAddress) constant returns(bool)
func (_Nodelist *NodelistCaller) ViewWhitelist(opts *bind.CallOpts, epoch *big.Int, nodeAddress common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _Nodelist.contract.Call(opts, out, "viewWhitelist", epoch, nodeAddress)
	return *ret0, err
}

// ViewWhitelist is a free data retrieval call binding the contract method 0x22d1b9db.
//
// Solidity: function viewWhitelist(uint256 epoch, address nodeAddress) constant returns(bool)
func (_Nodelist *NodelistSession) ViewWhitelist(epoch *big.Int, nodeAddress common.Address) (bool, error) {
	return _Nodelist.Contract.ViewWhitelist(&_Nodelist.CallOpts, epoch, nodeAddress)
}

// ViewWhitelist is a free data retrieval call binding the contract method 0x22d1b9db.
//
// Solidity: function viewWhitelist(uint256 epoch, address nodeAddress) constant returns(bool)
func (_Nodelist *NodelistCallerSession) ViewWhitelist(epoch *big.Int, nodeAddress common.Address) (bool, error) {
	return _Nodelist.Contract.ViewWhitelist(&_Nodelist.CallOpts, epoch, nodeAddress)
}

// ListNode is a paid mutator transaction binding the contract method 0xbf2d6f81.
//
// Solidity: function listNode(uint256 epoch, string declaredIp, uint256 pubKx, uint256 pubKy, string tmP2PListenAddress, string p2pListenAddress) returns()
func (_Nodelist *NodelistTransactor) ListNode(opts *bind.TransactOpts, epoch *big.Int, declaredIp string, pubKx *big.Int, pubKy *big.Int, tmP2PListenAddress string, p2pListenAddress string) (*types.Transaction, error) {
	return _Nodelist.contract.Transact(opts, "listNode", epoch, declaredIp, pubKx, pubKy, tmP2PListenAddress, p2pListenAddress)
}

// ListNode is a paid mutator transaction binding the contract method 0xbf2d6f81.
//
// Solidity: function listNode(uint256 epoch, string declaredIp, uint256 pubKx, uint256 pubKy, string tmP2PListenAddress, string p2pListenAddress) returns()
func (_Nodelist *NodelistSession) ListNode(epoch *big.Int, declaredIp string, pubKx *big.Int, pubKy *big.Int, tmP2PListenAddress string, p2pListenAddress string) (*types.Transaction, error) {
	return _Nodelist.Contract.ListNode(&_Nodelist.TransactOpts, epoch, declaredIp, pubKx, pubKy, tmP2PListenAddress, p2pListenAddress)
}

// ListNode is a paid mutator transaction binding the contract method 0xbf2d6f81.
//
// Solidity: function listNode(uint256 epoch, string declaredIp, uint256 pubKx, uint256 pubKy, string tmP2PListenAddress, string p2pListenAddress) returns()
func (_Nodelist *NodelistTransactorSession) ListNode(epoch *big.Int, declaredIp string, pubKx *big.Int, pubKy *big.Int, tmP2PListenAddress string, p2pListenAddress string) (*types.Transaction, error) {
	return _Nodelist.Contract.ListNode(&_Nodelist.TransactOpts, epoch, declaredIp, pubKx, pubKy, tmP2PListenAddress, p2pListenAddress)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Nodelist *NodelistTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Nodelist.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Nodelist *NodelistSession) RenounceOwnership() (*types.Transaction, error) {
	return _Nodelist.Contract.RenounceOwnership(&_Nodelist.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Nodelist *NodelistTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _Nodelist.Contract.RenounceOwnership(&_Nodelist.TransactOpts)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Nodelist *NodelistTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _Nodelist.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Nodelist *NodelistSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Nodelist.Contract.TransferOwnership(&_Nodelist.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Nodelist *NodelistTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Nodelist.Contract.TransferOwnership(&_Nodelist.TransactOpts, newOwner)
}

// UpdateWhitelist is a paid mutator transaction binding the contract method 0x3d4602a9.
//
// Solidity: function updateWhitelist(uint256 epoch, address nodeAddress, bool allowed) returns()
func (_Nodelist *NodelistTransactor) UpdateWhitelist(opts *bind.TransactOpts, epoch *big.Int, nodeAddress common.Address, allowed bool) (*types.Transaction, error) {
	return _Nodelist.contract.Transact(opts, "updateWhitelist", epoch, nodeAddress, allowed)
}

// UpdateWhitelist is a paid mutator transaction binding the contract method 0x3d4602a9.
//
// Solidity: function updateWhitelist(uint256 epoch, address nodeAddress, bool allowed) returns()
func (_Nodelist *NodelistSession) UpdateWhitelist(epoch *big.Int, nodeAddress common.Address, allowed bool) (*types.Transaction, error) {
	return _Nodelist.Contract.UpdateWhitelist(&_Nodelist.TransactOpts, epoch, nodeAddress, allowed)
}

// UpdateWhitelist is a paid mutator transaction binding the contract method 0x3d4602a9.
//
// Solidity: function updateWhitelist(uint256 epoch, address nodeAddress, bool allowed) returns()
func (_Nodelist *NodelistTransactorSession) UpdateWhitelist(epoch *big.Int, nodeAddress common.Address, allowed bool) (*types.Transaction, error) {
	return _Nodelist.Contract.UpdateWhitelist(&_Nodelist.TransactOpts, epoch, nodeAddress, allowed)
}

// NodelistNodeListedIterator is returned from FilterNodeListed and is used to iterate over the raw logs and unpacked data for NodeListed events raised by the Nodelist contract.
type NodelistNodeListedIterator struct {
	Event *NodelistNodeListed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NodelistNodeListedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NodelistNodeListed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NodelistNodeListed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NodelistNodeListedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NodelistNodeListedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NodelistNodeListed represents a NodeListed event raised by the Nodelist contract.
type NodelistNodeListed struct {
	PublicKey common.Address
	Epoch     *big.Int
	Position  *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterNodeListed is a free log retrieval operation binding the contract event 0xe2f8adb0f494dc82ccf446c031763ef3762d6396d51664611ed89aac0117339e.
//
// Solidity: event NodeListed(address publicKey, uint256 epoch, uint256 position)
func (_Nodelist *NodelistFilterer) FilterNodeListed(opts *bind.FilterOpts) (*NodelistNodeListedIterator, error) {

	logs, sub, err := _Nodelist.contract.FilterLogs(opts, "NodeListed")
	if err != nil {
		return nil, err
	}
	return &NodelistNodeListedIterator{contract: _Nodelist.contract, event: "NodeListed", logs: logs, sub: sub}, nil
}

// WatchNodeListed is a free log subscription operation binding the contract event 0xe2f8adb0f494dc82ccf446c031763ef3762d6396d51664611ed89aac0117339e.
//
// Solidity: event NodeListed(address publicKey, uint256 epoch, uint256 position)
func (_Nodelist *NodelistFilterer) WatchNodeListed(opts *bind.WatchOpts, sink chan<- *NodelistNodeListed) (event.Subscription, error) {

	logs, sub, err := _Nodelist.contract.WatchLogs(opts, "NodeListed")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NodelistNodeListed)
				if err := _Nodelist.contract.UnpackLog(event, "NodeListed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// NodelistOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the Nodelist contract.
type NodelistOwnershipTransferredIterator struct {
	Event *NodelistOwnershipTransferred // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NodelistOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NodelistOwnershipTransferred)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NodelistOwnershipTransferred)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NodelistOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NodelistOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NodelistOwnershipTransferred represents a OwnershipTransferred event raised by the Nodelist contract.
type NodelistOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Nodelist *NodelistFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*NodelistOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Nodelist.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &NodelistOwnershipTransferredIterator{contract: _Nodelist.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Nodelist *NodelistFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *NodelistOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Nodelist.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NodelistOwnershipTransferred)
				if err := _Nodelist.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}
