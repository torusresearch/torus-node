const NodeList = artifacts.require("./NodeList.sol");

module.exports = async function(deployer) {
  await deployer.deploy(NodeList);
};
