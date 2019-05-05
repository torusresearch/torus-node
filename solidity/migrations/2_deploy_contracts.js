var NodeList = artifacts.require('./NodeList.sol');

function timeout(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

var sleep = milliseconds => {
  return new Promise((resolve, reject) => {
    setTimeout(function() {
      resolve()
    }, milliseconds)
  })
}

module.exports = async function(deployer) {
  var isdone = false
  await deployer.deploy(NodeList);
  let nodeList = await NodeList.deployed();
  let c = new web3.eth.Contract(nodeList.abi, nodeList.address);
  //THIS IS NECESSARY BECAUSE TRUFFLE IS STUPID
  web3.eth.defaultAccount = web3.eth.accounts[0];
  var whitelistedAccounts = [
    "0x52C476751142Ce2FB4DB4f19B500e78FEEe10B06",
    "0xFF364b6b86eA5a4f59Cc4989dA23b833daC15304",
    "0xDC0dd04AaC998E8aA9F2de236b3bA04DDaFd26Ca",
    "0x253db77F1aE216722b2F67f33Ef3c8e00B2689e6",
    "0x271346169993368F94cB2c443B8b8CdbDD5EdF04",
    "0xa0ae28Ec27FEa7A577B21330f6CE8aE45A55Fe76",
    "0xf34a875cfFe643D44546b76F0c9412dFb9d2b379",
    "0x35d946c9c4598CD2eAEe5754CE2041911dc816Ce",
    "0xd6ee5E06Ac11a62fd0bE1912dEBEEB4abc24F723",
    "0x40fA4B9e4411E7F5f58713efF426CAD4F0294Ab5",
    "0x0cDa757357158e4d8ad94433e36f1Fe05A1dC576",
    "0xa22e3c16264dc688107142776139d1fB4BB9d549",
    "0x0b998B7229BFd254Acf50b4e2739E73D937Dc1c9",
    "0xFC54C26E24b4570590c11486bD627Aa4B7339523",
    "0xb572081928B988ABE713ffe60F8cf28ef80Eee07",
    "0xd54E0C310a97916E67d07aa501F74524e82c3af1",
    "0xABA31E255B490365584a56F4EbC5037963E584D5",
    "0x3EcEFaFEa7dB9d0E26dc0D266504587cb66f6008",
    "0x184b56D50300b4cd604A587491cb7BcB0Ffc7454",
    "0xD6eca392adA22e18c9EEbdE2828b38E66813af5f"
  ];
  
  c.methods.updateEpoch(1, 5, 3, 1, [], 0, 2)
  for (var i = 0; i < whitelistedAccounts.length; i++) {
    const acc = whitelistedAccounts[i];
    // await web3.sendTransaction({ to: acc, value: web3.toWei('1', 'ether') });
    console.log('adding', acc, ' to whitelist');
    c.methods.updateWhitelist(1, acc, true);
  }
  isdone = true
}
