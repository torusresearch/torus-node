pragma solidity  >=0.5.0 <0.7.0;

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

  struct Epoch {
    uint256 id;
    uint256 n;
    uint256 k;
    uint256 t;
    address[] nodeList;
    uint256 prevEpoch;
    uint256 nextEpoch;
  }

  mapping(uint256 => mapping (address => bool)) public whitelist;

  mapping (uint256 => Epoch) public epochInfo;

  mapping (address => Details) public nodeDetails;

  constructor() public {
  }

  function getNodes(uint256 epoch) external view epochValid(epoch) returns (address[] memory) {
    return epochInfo[epoch].nodeList;
  }

  function getNodeDetails(address nodeAddress) external view returns (string memory declaredIp, uint256 position, 
    string memory tmP2PListenAddress, string memory p2pListenAddress) {
    Details memory nodeDetail; 
    nodeDetail = nodeDetails[nodeAddress];
    return (nodeDetail.declaredIp, nodeDetail.position, nodeDetail.tmP2PListenAddress, nodeDetail.p2pListenAddress);
  }

  modifier epochValid(uint256 epoch) {
    require(epoch != 0);
    _;
  }

  modifier epochCreated(uint256 epoch) {
    require(epochInfo[epoch].id == epoch);
    _;
  }

  modifier whitelisted(uint256 epoch) {
    require(IsWhitelisted(epoch, msg.sender));
    _;
  }

  function IsWhitelisted(uint256 epoch, address nodeAddress) public view returns (bool) {
    return whitelist[epoch][nodeAddress];
  }


  function updateWhitelist(uint256 epoch, address nodeAddress, bool allowed) public onlyOwner epochValid(epoch) {
    whitelist[epoch][nodeAddress] = allowed;
  }

  function updateEpoch(uint256 epoch, uint256 n, uint256 k, uint256 t, address[] memory nodeList, uint256 prevEpoch, uint256 nextEpoch) 
    public onlyOwner epochValid(epoch) {
      epochInfo[epoch] = Epoch(epoch, n, k, t, nodeList, prevEpoch, nextEpoch);
    }

  function getEpochInfo(uint256 epoch) public view epochValid(epoch) returns (uint256 id, uint256 n, uint256 k, uint256 t, 
    address[] memory nodeList, uint256 prevEpoch, uint256 nextEpoch) {
    Epoch memory epochI = epochInfo[epoch];
    return (epochI.id, epochI.n, epochI.k, epochI.t, epochI.nodeList, epochI.prevEpoch, epochI.nextEpoch);
  }

  function listNode(uint256 epoch, string calldata declaredIp, uint256 pubKx, uint256 pubKy, string calldata tmP2PListenAddress, 
    string calldata p2pListenAddress) external whitelisted(epoch) epochValid(epoch) epochCreated(epoch) {
    Epoch storage epochI = epochInfo[epoch];
    epochI.nodeList.push(msg.sender); 
    nodeDetails[msg.sender] = Details({
      declaredIp: declaredIp,
      position: epochI.nodeList.length,
      pubKx: pubKx,
      pubKy: pubKy,
      tmP2PListenAddress: tmP2PListenAddress,
      p2pListenAddress: p2pListenAddress
    });
    emit NodeListed(msg.sender, epoch, epochI.nodeList.length);
  }
}