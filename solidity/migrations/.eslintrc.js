module.exports = {
  env: {
    mocha: true
  },
  globals: {
    artifacts: true,
    contract: true,
    assert: true,
    web3: true
  },
  rules: {
    "padded-blocks": 0,
    "no-unused-expressions": 0,
    "no-unused-vars": 1,
    "newline-per-chained-call": 0
  }
};
