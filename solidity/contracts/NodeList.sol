pragma solidity >=0.5.0 <=0.5.15;

import "./Ownable.sol";

contract NodeList is Ownable {
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

    event NodeListed(address publicKey, uint256 epoch, uint256 position);

    mapping(uint256 => mapping(address => bool)) public whitelist;

    mapping(uint256 => Epoch) public epochInfo;

    mapping(address => Details) public nodeDetails;

    mapping(uint256 => mapping(uint256 => uint256)) public pssStatus;

    modifier epochValid(uint256 epoch) {
        require(epoch != 0, "Epoch can't be 0");
        _;
    }

    modifier epochCreated(uint256 epoch) {
        require(epochInfo[epoch].id == epoch, "Epoch already created");
        _;
    }

    modifier whitelisted(uint256 epoch) {
        require(isWhitelisted(epoch, msg.sender), "Node isn't whitelisted for epoch");
        _;
    }

    function listNode(
        uint256 epoch,
        string calldata declaredIp,
        uint256 pubKx,
        uint256 pubKy,
        string calldata tmP2PListenAddress,
        string calldata p2pListenAddress
    ) external whitelisted(epoch) epochValid(epoch) epochCreated(epoch) {
        require(!nodeRegistered(epoch, msg.sender), "Node is already registered");
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

    function getNodes(uint256 epoch) external view epochValid(epoch) returns (address[] memory) {
        return epochInfo[epoch].nodeList;
    }

    function getNodeDetails(address nodeAddress)
        external
        view
        returns (string memory declaredIp, uint256 position, string memory tmP2PListenAddress, string memory p2pListenAddress)
    {
        Details memory nodeDetail;
        nodeDetail = nodeDetails[nodeAddress];
        return (nodeDetail.declaredIp, nodeDetail.position, nodeDetail.tmP2PListenAddress, nodeDetail.p2pListenAddress);
    }

    function getPssStatus(uint256 oldEpoch, uint256 newEpoch) external view returns (uint256) {
        return pssStatus[oldEpoch][newEpoch];
    }

    function getEpochInfo(uint256 epoch)
        external
        view
        epochValid(epoch)
        returns (uint256 id, uint256 n, uint256 k, uint256 t, address[] memory nodeList, uint256 prevEpoch, uint256 nextEpoch)
    {
        Epoch memory epochI = epochInfo[epoch];
        return (epochI.id, epochI.n, epochI.k, epochI.t, epochI.nodeList, epochI.prevEpoch, epochI.nextEpoch);
    }

    function updatePssStatus(uint256 oldEpoch, uint256 newEpoch, uint256 status) public onlyOwner epochValid(oldEpoch) epochValid(newEpoch) {
        pssStatus[oldEpoch][newEpoch] = status;
    }

    function updateWhitelist(uint256 epoch, address nodeAddress, bool allowed) public onlyOwner epochValid(epoch) {
        whitelist[epoch][nodeAddress] = allowed;
    }

    function updateEpoch(uint256 epoch, uint256 n, uint256 k, uint256 t, address[] memory nodeList, uint256 prevEpoch, uint256 nextEpoch)
        public
        onlyOwner
        epochValid(epoch)
    {
        epochInfo[epoch] = Epoch(epoch, n, k, t, nodeList, prevEpoch, nextEpoch);
    }

    function isWhitelisted(uint256 epoch, address nodeAddress) public view returns (bool) {
        return whitelist[epoch][nodeAddress];
    }

    function nodeRegistered(uint256 epoch, address nodeAddress) public view returns (bool) {
        Epoch storage epochI = epochInfo[epoch];
        for (uint256 i = 0; i < epochI.nodeList.length; i++) {
            if (epochI.nodeList[i] == nodeAddress) {
                return true;
            }
        }
        return false;
    }
}
