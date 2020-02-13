pragma solidity >=0.5.0 <=0.5.15;

import "./Ownable.sol";

/**
 * @title Proxy
 * @dev Basic proxy that delegates all calls to a fixed implementing contract.
 * The implementing contract cannot be upgraded.
 */
contract Proxy is Ownable {
    address public implementation;

    event Received(uint256 indexed value, address indexed sender, bytes data);
    event ChangedImplementationContract(address implementation);

    constructor(address _implementation) public {
        implementation = _implementation;
    }

    function() external payable {
        address target = implementation;
        // solhint-disable-next-line no-inline-assembly
        assembly {
            calldatacopy(0, 0, calldatasize)
            let result := call(gas, target, callvalue, 0, calldatasize, 0, 0)
            returndatacopy(0, 0, returndatasize)
            switch result
                case 0 {
                    revert(0, returndatasize)
                }
                default {
                    return(0, returndatasize)
                }
        }
    }

    function setImplementation(address _implementation) external onlyOwner {
        require(_implementation != address(0), "Can't set zero address");
        implementation = _implementation;
        emit ChangedImplementationContract(_implementation);
    }
}
