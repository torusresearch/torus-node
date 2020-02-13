const NodeListProxy = artifacts.require("./NodeListProxy.sol");
const NodeList = artifacts.require("NodeList.sol");
const whitelistedAccounts = require("../whiteList");

module.exports = async function(deployer) {
  await deployer.deploy(NodeListProxy, NodeList.address, 1);
  const nodeListProxyInstance = await NodeListProxy.at(NodeListProxy.address);
  var res = await nodeListProxyInstance.isWhitelisted(1, whitelistedAccounts[0]);
  console.log("should be whitelisted, isWhitelisted: ", res);

  var res2 = await nodeListProxyInstance.getEpochInfo(1);
  console.log("epoch info: ", res2);

  await nodeListProxyInstance.setCurrentEpoch(2);
  var res3 = await nodeListProxyInstance.currentEpoch();
  console.log("epoch set", res3);
};
