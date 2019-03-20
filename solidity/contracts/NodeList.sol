pragma solidity ^0.4.24;

/**
 * @title Ownable
 * @dev The Ownable contract has an owner address, and provides basic authorization control
 * functions, this simplifies the implementation of "user permissions".
 */
contract Ownable {
    address private _owner;

    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    /**
     * @dev The Ownable constructor sets the original `owner` of the contract to the sender
     * account.
     */
    constructor () internal {
        _owner = msg.sender;
        emit OwnershipTransferred(address(0), _owner);
    }

    /**
     * @return the address of the owner.
     */
    function owner() public view returns (address) {
        return _owner;
    }

    /**
     * @dev Throws if called by any account other than the owner.
     */
    modifier onlyOwner() {
        require(isOwner());
        _;
    }

    /**
     * @return true if `msg.sender` is the owner of the contract.
     */
    function isOwner() public view returns (bool) {
        return msg.sender == _owner;
    }

    /**
     * @dev Allows the current owner to relinquish control of the contract.
     * @notice Renouncing to ownership will leave the contract without an owner.
     * It will not be possible to call the functions with the `onlyOwner`
     * modifier anymore.
     */
    function renounceOwnership() public onlyOwner {
        emit OwnershipTransferred(_owner, address(0));
        _owner = address(0);
    }

    /**
     * @dev Allows the current owner to transfer control of the contract to a newOwner.
     * @param newOwner The address to transfer ownership to.
     */
    function transferOwnership(address newOwner) public onlyOwner {
        _transferOwnership(newOwner);
    }

    /**
     * @dev Transfers control of the contract to a newOwner.
     * @param newOwner The address to transfer ownership to.
     */
    function _transferOwnership(address newOwner) internal {
        require(newOwner != address(0));
        emit OwnershipTransferred(_owner, newOwner);
        _owner = newOwner;
    }
}

contract NodeList is Ownable {
  event NodeListed(address publicKey, uint256 epoch, uint256 position);

  struct Details {
    string declaredIp;
    uint256 position;
    uint256 pubKx;
    uint256 pubKy;
    string tmP2PListenAddress;
    string p2pListenAddress;
  }

  mapping (uint256 => mapping (address => bool)) whitelist;

  mapping (address => mapping (uint256 => Details)) public addressToNodeDetailsLog; //mapping of address => epoch => nodeDetailsLog
  mapping (uint256 => address[]) public nodeList; // mapping of epoch => list of nodes in epoch
  uint256 latestEpoch = 0; //count of number of epochs

  constructor() public {
  }

  //views nodes in the epoch, now requires specified epochs
  function viewNodes(uint256 epoch) external view  returns (address[], uint256[]) {
    uint256[] memory positions = new uint256[](nodeList[epoch].length);
    for (uint256 i = 0; i < nodeList[epoch].length; i++) {
      positions[i] = addressToNodeDetailsLog[nodeList[epoch][i]][epoch].position;
    }
    return (nodeList[epoch], positions);
  }

  function viewNodeListCount(uint256 epoch) external view returns (uint256) {
    return nodeList[epoch].length;
  }

  function viewLatestEpoch() external view returns (uint256) {
    return latestEpoch;
  }

  function viewNodeDetails(uint256 epoch, address node) external view  returns (string declaredIp, uint256 position, string tmP2PListenAddress, string p2pListenAddress) {
    declaredIp = addressToNodeDetailsLog[node][epoch].declaredIp;
    position = addressToNodeDetailsLog[node][epoch].position;
    tmP2PListenAddress = addressToNodeDetailsLog[node][epoch].tmP2PListenAddress;
    p2pListenAddress = addressToNodeDetailsLog[node][epoch].p2pListenAddress;
  }

  function viewWhitelist(uint256 epoch, address nodeAddress) public view returns (bool) {
    return whitelist[epoch][nodeAddress];
  }

  modifier whitelisted(uint256 epoch) {
    require(whitelist[epoch][msg.sender]);
    _;
  }

  function updateWhitelist(uint256 epoch, address nodeAddress, bool allowed) public onlyOwner {
    whitelist[epoch][nodeAddress] = allowed;
  }

  function listNode(uint256 epoch, string declaredIp, uint256 pubKx, uint256 pubKy, string tmP2PListenAddress, string p2pListenAddress) external whitelisted(epoch) {
    nodeList[epoch].push(msg.sender); 
    addressToNodeDetailsLog[msg.sender][epoch] = Details({
      declaredIp: declaredIp,
      position: nodeList[epoch].length, //so that Position (or node index) starts from 1
      pubKx: pubKx,
      pubKy: pubKy,
      tmP2PListenAddress: tmP2PListenAddress,
      p2pListenAddress: p2pListenAddress
      });
    //for now latest epoch is simply the highest epoch registered TODO: only we should be able to call this function
    if (latestEpoch < epoch) {
      latestEpoch = epoch;
    }
    emit NodeListed(msg.sender, epoch, nodeList[epoch].length);
  }
}