pragma solidity >=0.5.0 <=0.5.15;

import "./Ownable.sol";
import "./NodeList.sol";

contract NodeListProxy is Ownable {
    uint256 public currentEpoch;
    NodeList private nodeListContract;

    event EpochChanged(uint256 oldEpoch, uint256 newEpoch);
    event NodeListContractChanged(address oldContract, address newContract);

    constructor(address nodeListContractAddress, uint256 epoch) public {
        currentEpoch = epoch;
        nodeListContract = NodeList(nodeListContractAddress);
    }

    function setCurrentEpoch(uint256 _newEpoch) external onlyOwner {
        uint256 oldEpoch = currentEpoch;
        currentEpoch = _newEpoch;
        emit EpochChanged(oldEpoch, _newEpoch);
    }

    function setNodeListContract(address nodeListContractAddress) external onlyOwner {
        require(nodeListContractAddress != address(0), "no zero address");
        address oldAddress = address(nodeListContract);
        nodeListContract = NodeList(nodeListContractAddress);
        emit NodeListContractChanged(oldAddress, nodeListContractAddress);
    }

    function getNodes(uint256 epoch) external view returns (address[] memory) {
        return nodeListContract.getNodes(epoch);
    }

    function getNodeDetails(address nodeAddress)
        external
        view
        returns (
            string memory declaredIp,
            uint256 position,
            uint256 pubKx,
            uint256 pubKy,
            string memory tmP2PListenAddress,
            string memory p2pListenAddress
        )
    {
        return nodeListContract.nodeDetails(nodeAddress);
    }

    function getPssStatus(uint256 oldEpoch, uint256 newEpoch) external view returns (uint256) {
        return nodeListContract.getPssStatus(oldEpoch, newEpoch);
    }

    function isWhitelisted(uint256 epoch, address nodeAddress) external view returns (bool) {
        return nodeListContract.isWhitelisted(epoch, nodeAddress);
    }

    function getEpochInfo(uint256 epoch)
        external
        view
        returns (uint256 id, uint256 n, uint256 k, uint256 t, address[] memory nodeList, uint256 prevEpoch, uint256 nextEpoch)
    {
        return nodeListContract.getEpochInfo(epoch);
    }
}
