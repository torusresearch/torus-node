const ProxyContract = artifacts.require("./Proxy.sol");
const NodeListProxy = artifacts.require("NodeListProxy.sol");

module.exports = async function(deployer, network, accounts) {
  await deployer.deploy(ProxyContract, NodeListProxy.address);
  const nodeListInstance = await NodeListProxy.at(NodeListProxy.address);
  const proxySend = await nodeListInstance.contract.methods.getEpochInfo(1).encodeABI();
  const finalProxySend = await web3.eth.call({data: proxySend, from: accounts[0], to: ProxyContract.address});
  console.log(finalProxySend, "final proxy send");
  const response = await nodeListInstance.getEpochInfo(1);
  console.log(response, "checking for updated epoch info");
};
