var NodeList = artifacts.require("NodeList.sol")

contract('Node List', async (accounts) => {
  beforeEach(async () => {
    nodeList = await NodeList.new()
  })

  it("should whitelist", async () => {
    await nodeList.updateWhitelist(1, "0x3ecefafea7db9d0e26dc0d266504587cb66f6008", true)
    result = await nodeList.IsWhitelisted(1, "0x3ecefafea7db9d0e26dc0d266504587cb66f6008")
    assert(result === true)
  })
})
