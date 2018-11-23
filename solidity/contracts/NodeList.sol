pragma solidity ^0.4.24;

contract NodeList {
  event NodeListed(address publicKey, uint256 position);

  struct Details {
    string declaredIp;
    uint256 position;
    uint256 pubKx;
    uint256 pubKy;
  }

  mapping (address => Details) public nodeDetails;
  address[] public nodeList;

  constructor() public {
  }


  function viewNodes() external view  returns (address[], uint256[]) {
    uint256[] memory positions = new uint256[](nodeList.length);
    for (uint256 i = 0; i < nodeList.length; i++) {
      positions[i] = nodeDetails[nodeList[i]].position;
    }
    return (nodeList, positions);
  }

  function viewNodeListCount() external view  returns (uint256) {
    return nodeList.length;
  }

  function viewNodeDetails(address node) external view  returns (string declaredIp, uint256 position) {
    declaredIp = nodeDetails[node].declaredIp;
    position = nodeDetails[node].position;
  }

  function listNode(string declaredIp, uint256 pubKx, uint256 pubKy) external {
    nodeList.push(msg.sender);
    nodeDetails[msg.sender] = Details({declaredIp: declaredIp, position: nodeList.length, pubKx: pubKx, pubKy: pubKy});
    emit NodeListed(msg.sender, nodeList.length);
  }
}