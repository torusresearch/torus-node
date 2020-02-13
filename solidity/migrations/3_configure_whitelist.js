const NodeList = artifacts.require("./NodeList.sol");
const whitelistedAccounts = require("../whiteList");

function tx(result, call) {
  const logs = result.logs.length > 0 ? result.logs[0] : {address: null, event: null};

  console.log();
  console.log(`   Calling ${call}`);
  console.log("   ------------------------");
  console.log(`   > transaction hash: ${result.tx}`);
  console.log(`   > contract address: ${logs.address}`);
  console.log(`   > gas used: ${result.receipt.gasUsed}`);
  console.log(`   > event: ${logs.event}`);
  console.log();
}

module.exports = async function(deployer, network, accounts) {
  const NodeListInstance = await NodeList.at(NodeList.address);
  tx(await NodeListInstance.updateEpoch(1, 5, 3, 1, [accounts[0], accounts[2]], 0, 2), "Updated epoch");
  for (let i = 0; i < whitelistedAccounts.length; i++) {
    const acc = whitelistedAccounts[i];
    tx(await NodeListInstance.updateWhitelist(1, acc, true, {from: accounts[0], gas: "100000"}), `adding ${acc} to whitelist`);
  }

  for (var i = 0; i < whitelistedAccounts.length; i++) {
    var res = await NodeListInstance.isWhitelisted(1, whitelistedAccounts[i]);
    console.log("should be whitelisted, isWhitelisted: ", res);
  }
};
