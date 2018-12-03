var NodeList = artifacts.require("./NodeList.sol");

module.exports = async function(deployer) {
  await deployer.deploy(NodeList);

  let nodeList = await NodeList.deployed()
  debugger
  let c = web3.eth.contract(nodeList.abi).at(nodeList.address)
  web3.eth.defaultAccount = web3.eth.accounts[0]
  for (var i = 0; i < web3.eth.accounts.length; i++) {
    await c.updateWhiteList(0, web3.eth.accounts[i], true)
  }
};


