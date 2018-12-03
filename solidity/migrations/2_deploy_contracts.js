var NodeList = artifacts.require("./NodeList.sol");

function timeout(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

module.exports = async function(deployer) {
  await deployer.deploy(NodeList);
  let nodeList = await NodeList.deployed()
  let c = web3.eth.contract(nodeList.abi).at(nodeList.address)
  web3.eth.defaultAccount = web3.eth.accounts[0]
  for (var i = 0; i < web3.eth.accounts.length; i++) {
    await c.updateWhitelist(0, web3.eth.accounts[i], true)
  }
};


