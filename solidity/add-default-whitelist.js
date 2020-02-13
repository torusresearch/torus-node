const whitelistedAccounts = require("./whiteList");
const NodeList = artifacts.require("./NodeList.sol");

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

// How to run: truffle exec --network development ./add-default-whitelist.js

module.exports = async callback => {
  try {
    const accounts = await web3.eth.getAccounts();
    // Assuming that we need to deploy first and then add the whitelisted accounts
    const NodeListInstance = await NodeList.new();
    // if already deployed, uncomment next line and comment the above
    // const NodeListInstance = await NodeList.at(NodeList.address)
    console.log("Node List Contract: ", NodeListInstance.address);

    for (let i = 0; i < whitelistedAccounts.length; i++) {
      const acc = whitelistedAccounts[i];
      tx(await NodeListInstance.updateWhitelist(0, acc, true, {from: accounts[0], gas: "100000"}), `adding ${acc} to whitelist`);
    }
    // can also use promise.all and send all at once
    callback();
  } catch (error) {
    callback(error);
  }
};
