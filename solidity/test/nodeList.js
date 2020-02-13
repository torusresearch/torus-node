var NodeList = artifacts.require("./NodeList.sol");

contract("Node List", function(accounts) {
  let NodeListInstance;
  // console.log('testing')
  // beforeEach(async () => {
  //   console.log('deployed')
  // })

  it("should  list()", async () => {
    //   console.log('deploying')
    NodeListInstance = await NodeList.deployed();
    await NodeListInstance.updateWhitelist(1, accounts[0], true);
    const result = await NodeListInstance.sWhitelisted(1, accounts[0]);
    assert(result === true);
    // console.log('testing')
  });
});
