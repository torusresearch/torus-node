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
const NodelistABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"node\",\"type\":\"address\"}],\"name\":\"viewNodeDetails\",\"outputs\":[{\"name\":\"declaredIp\",\"type\":\"string\"},{\"name\":\"position\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"nodeList\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"viewNodeListCount\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"viewNodeList\",\"outputs\":[{\"name\":\"\",\"type\":\"address[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"nodeDetails\",\"outputs\":[{\"name\":\"declaredIp\",\"type\":\"string\"},{\"name\":\"position\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"declaredIp\",\"type\":\"string\"}],\"name\":\"listNode\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"publicKey\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"position\",\"type\":\"uint256\"}],\"name\":\"NodeListed\",\"type\":\"event\"}]"

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

// NodeDetails is a free data retrieval call binding the contract method 0x859da85f.
//
// Solidity: function nodeDetails( address) constant returns(declaredIp string, position uint256)
func (_Nodelist *NodelistCaller) NodeDetails(opts *bind.CallOpts, arg0 common.Address) (struct {
	DeclaredIp string
	Position   *big.Int
}, error) {
	ret := new(struct {
		DeclaredIp string
		Position   *big.Int
	})
	out := ret
	err := _Nodelist.contract.Call(opts, out, "nodeDetails", arg0)
	return *ret, err
}

// NodeDetails is a free data retrieval call binding the contract method 0x859da85f.
//
// Solidity: function nodeDetails( address) constant returns(declaredIp string, position uint256)
func (_Nodelist *NodelistSession) NodeDetails(arg0 common.Address) (struct {
	DeclaredIp string
	Position   *big.Int
}, error) {
	return _Nodelist.Contract.NodeDetails(&_Nodelist.CallOpts, arg0)
}

// NodeDetails is a free data retrieval call binding the contract method 0x859da85f.
//
// Solidity: function nodeDetails( address) constant returns(declaredIp string, position uint256)
func (_Nodelist *NodelistCallerSession) NodeDetails(arg0 common.Address) (struct {
	DeclaredIp string
	Position   *big.Int
}, error) {
	return _Nodelist.Contract.NodeDetails(&_Nodelist.CallOpts, arg0)
}

// NodeList is a free data retrieval call binding the contract method 0x208f2a31.
//
// Solidity: function nodeList( uint256) constant returns(address)
func (_Nodelist *NodelistCaller) NodeList(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _Nodelist.contract.Call(opts, out, "nodeList", arg0)
	return *ret0, err
}

// NodeList is a free data retrieval call binding the contract method 0x208f2a31.
//
// Solidity: function nodeList( uint256) constant returns(address)
func (_Nodelist *NodelistSession) NodeList(arg0 *big.Int) (common.Address, error) {
	return _Nodelist.Contract.NodeList(&_Nodelist.CallOpts, arg0)
}

// NodeList is a free data retrieval call binding the contract method 0x208f2a31.
//
// Solidity: function nodeList( uint256) constant returns(address)
func (_Nodelist *NodelistCallerSession) NodeList(arg0 *big.Int) (common.Address, error) {
	return _Nodelist.Contract.NodeList(&_Nodelist.CallOpts, arg0)
}

// ViewNodeDetails is a free data retrieval call binding the contract method 0x04071857.
//
// Solidity: function viewNodeDetails(node address) constant returns(declaredIp string, position uint256)
func (_Nodelist *NodelistCaller) ViewNodeDetails(opts *bind.CallOpts, node common.Address) (struct {
	DeclaredIp string
	Position   *big.Int
}, error) {
	ret := new(struct {
		DeclaredIp string
		Position   *big.Int
	})
	out := ret
	err := _Nodelist.contract.Call(opts, out, "viewNodeDetails", node)
	return *ret, err
}

// ViewNodeDetails is a free data retrieval call binding the contract method 0x04071857.
//
// Solidity: function viewNodeDetails(node address) constant returns(declaredIp string, position uint256)
func (_Nodelist *NodelistSession) ViewNodeDetails(node common.Address) (struct {
	DeclaredIp string
	Position   *big.Int
}, error) {
	return _Nodelist.Contract.ViewNodeDetails(&_Nodelist.CallOpts, node)
}

// ViewNodeDetails is a free data retrieval call binding the contract method 0x04071857.
//
// Solidity: function viewNodeDetails(node address) constant returns(declaredIp string, position uint256)
func (_Nodelist *NodelistCallerSession) ViewNodeDetails(node common.Address) (struct {
	DeclaredIp string
	Position   *big.Int
}, error) {
	return _Nodelist.Contract.ViewNodeDetails(&_Nodelist.CallOpts, node)
}

// ViewNodeList is a free data retrieval call binding the contract method 0x7f7e1e36.
//
// Solidity: function viewNodeList() constant returns(address[])
func (_Nodelist *NodelistCaller) ViewNodeList(opts *bind.CallOpts) ([]common.Address, error) {
	var (
		ret0 = new([]common.Address)
	)
	out := ret0
	err := _Nodelist.contract.Call(opts, out, "viewNodeList")
	return *ret0, err
}

// ViewNodeList is a free data retrieval call binding the contract method 0x7f7e1e36.
//
// Solidity: function viewNodeList() constant returns(address[])
func (_Nodelist *NodelistSession) ViewNodeList() ([]common.Address, error) {
	return _Nodelist.Contract.ViewNodeList(&_Nodelist.CallOpts)
}

// ViewNodeList is a free data retrieval call binding the contract method 0x7f7e1e36.
//
// Solidity: function viewNodeList() constant returns(address[])
func (_Nodelist *NodelistCallerSession) ViewNodeList() ([]common.Address, error) {
	return _Nodelist.Contract.ViewNodeList(&_Nodelist.CallOpts)
}

// ViewNodeListCount is a free data retrieval call binding the contract method 0x48eec1ec.
//
// Solidity: function viewNodeListCount() constant returns(uint256)
func (_Nodelist *NodelistCaller) ViewNodeListCount(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _Nodelist.contract.Call(opts, out, "viewNodeListCount")
	return *ret0, err
}

// ViewNodeListCount is a free data retrieval call binding the contract method 0x48eec1ec.
//
// Solidity: function viewNodeListCount() constant returns(uint256)
func (_Nodelist *NodelistSession) ViewNodeListCount() (*big.Int, error) {
	return _Nodelist.Contract.ViewNodeListCount(&_Nodelist.CallOpts)
}

// ViewNodeListCount is a free data retrieval call binding the contract method 0x48eec1ec.
//
// Solidity: function viewNodeListCount() constant returns(uint256)
func (_Nodelist *NodelistCallerSession) ViewNodeListCount() (*big.Int, error) {
	return _Nodelist.Contract.ViewNodeListCount(&_Nodelist.CallOpts)
}

// ListNode is a paid mutator transaction binding the contract method 0xed6f79ab.
//
// Solidity: function listNode(declaredIp string) returns()
func (_Nodelist *NodelistTransactor) ListNode(opts *bind.TransactOpts, declaredIp string) (*types.Transaction, error) {
	return _Nodelist.contract.Transact(opts, "listNode", declaredIp)
}

// ListNode is a paid mutator transaction binding the contract method 0xed6f79ab.
//
// Solidity: function listNode(declaredIp string) returns()
func (_Nodelist *NodelistSession) ListNode(declaredIp string) (*types.Transaction, error) {
	return _Nodelist.Contract.ListNode(&_Nodelist.TransactOpts, declaredIp)
}

// ListNode is a paid mutator transaction binding the contract method 0xed6f79ab.
//
// Solidity: function listNode(declaredIp string) returns()
func (_Nodelist *NodelistTransactorSession) ListNode(declaredIp string) (*types.Transaction, error) {
	return _Nodelist.Contract.ListNode(&_Nodelist.TransactOpts, declaredIp)
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
	Position  *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterNodeListed is a free log retrieval operation binding the contract event 0xefcfb17cd57f23e1f4a32d6e38cf2a608cccc7db00986211e0ea32e7336b230f.
//
// Solidity: e NodeListed(publicKey address, position uint256)
func (_Nodelist *NodelistFilterer) FilterNodeListed(opts *bind.FilterOpts) (*NodelistNodeListedIterator, error) {

	logs, sub, err := _Nodelist.contract.FilterLogs(opts, "NodeListed")
	if err != nil {
		return nil, err
	}
	return &NodelistNodeListedIterator{contract: _Nodelist.contract, event: "NodeListed", logs: logs, sub: sub}, nil
}

// WatchNodeListed is a free log subscription operation binding the contract event 0xefcfb17cd57f23e1f4a32d6e38cf2a608cccc7db00986211e0ea32e7336b230f.
//
// Solidity: e NodeListed(publicKey address, position uint256)
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
