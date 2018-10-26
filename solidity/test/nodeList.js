var NodeList = artifacts.require("NodeList.sol");

contract('Node List', async (accounts) => {
  beforeEach(async () => {
    nodeList = await NodeList.new();
  })

  it("should  list()", async () => {
    var result = await nodeList.listNode("yoyo", 1000, 1000);
    console.log(result.logs[0].args)
    assert(result.logs[0].event === "NodeListed", "should emit event nodelist");
    // result = await nodeList.viewNodeList()
    // result = await nodeList.viewNodeCount()
  })
})
