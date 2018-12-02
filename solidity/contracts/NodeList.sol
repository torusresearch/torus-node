pragma solidity ^0.4.24;

contract NodeList {
  event NodeListed(address publicKey, uint256 epoch, uint256 position);

  struct Details {
    string declaredIp;
    uint256 position;
    uint256 pubKx;
    uint256 pubKy;
    string nodePort;
  }

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

  function viewNodeListCount(uint256 epoch) external view  returns (uint256) {
    return nodeList[epoch].length;
  }

  function viewLatestEpoch() external view returns (uint256) {
    return latestEpoch;
  }

  function viewNodeDetails(uint256 epoch, address node) external view  returns (string declaredIp, uint256 position, string nodePort) {
    declaredIp = addressToNodeDetailsLog[node][epoch].declaredIp;
    position = addressToNodeDetailsLog[node][epoch].position;
    nodePort = addressToNodeDetailsLog[node][epoch].nodePort;
  }

  function listNode(uint256 epoch, string declaredIp, uint256 pubKx, uint256 pubKy, string nodePort) external {
    nodeList[epoch].push(msg.sender); 
    addressToNodeDetailsLog[msg.sender][epoch] = Details({
      declaredIp: declaredIp,
      position: nodeList[epoch].length, //so that Position (or node index) starts from 1
      pubKx: pubKx,
      pubKy: pubKy,
      nodePort: nodePort
      });
    //for now latest epoch is simply the highest epoch registered TODO: only we should be able to call this function
    if (latestEpoch < epoch) {
      latestEpoch = epoch;
    }
    emit NodeListed(msg.sender, epoch, nodeList[epoch].length);
  }
}