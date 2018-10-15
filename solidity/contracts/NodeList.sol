pragma solidity ^0.4.24;

contract NodeList {
  event NodeListed(address publicKey, uint256 position);

  struct Details {
    string declaredIp;
    uint256 position;
  }

  mapping (address => Details) public nodeDetails;
  address[] nodeList;

  constructor() public {
  }

  function viewNodeList() external view  returns (address[]) {
    return nodeList;
  }

  function viewNodeDetails(address node) external view  returns (string declaredIp, uint256 position) {
    declaredIp = nodeDetails[node].declaredIp;
    position = nodeDetails[node].position;
  }

  function listNode(string declaredIp) external {
    nodeList[nodeList.length] = msg.sender;
    nodeDetails[msg.sender] = Details({declaredIp: declaredIp, position: nodeList.length});
    emit NodeListed(msg.sender, nodeList.length);
  }
}