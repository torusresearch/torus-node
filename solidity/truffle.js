/*
 * NB: since truffle-hdwallet-provider 0.0.5 you must wrap HDWallet providers in a 
 * function when declaring them. Failure to do so will cause commands to hang. ex:
 * ```
 * mainnet: {
 *     provider: function() { 
 *       return new HDWalletProvider(mnemonic, 'https://mainnet.infura.io/<infura-key>') 
 *     },
 *     network_id: '1',
 *     gas: 4500000,
 *     gasPrice: 10000000000,
 *   },
 */

// provider: function () {
//   var wallet = new HDWalletProvider(mnemonic, `https://ropsten.infura.io/${infuraKey}`);
//   var nonceTracker = new NonceTrackerSubprovider();
//   wallet.engine._providers.unshift(nonceTracker);
//   nonceTracker.setEngine(wallet.engine);
//   return wallet;
// },
var NonceTrackerSubprovider = require("web3-provider-engine/subproviders/nonce-tracker")
const HDWalletProvider = require("truffle-hdwallet-provider");
const private = require('../private.json');
const mnemonic = private.funds;
const infuraKey = private.infura;

module.exports = {
  // See <http://truffleframework.com/docs/advanced/configuration>
  // to customize your Truffle configuration!
  networks: {
    development: {
       host: 'localhost',
       port: 14103,
       network_id: '*', // Match any network id
       gas: 4700000,
     },
    digital: {
      provider: new HDWalletProvider(private.ganache, 'http://178.128.178.162:14103'),
      network_id: '*',
      network_id: '*',
      gas: 4700000,
      gasPrice: 5000000000, // 50 gwei, this is very high
    },
    ropsten: {
      provider: new HDWalletProvider(mnemonic, `https://ropsten.infura.io/${infuraKey}`),
       network_id: '*',
       gas: 4700000,
       gasPrice: 5000000000, // 50 gwei, this is very high
     },
     rinkeby: {
        provider: new HDWalletProvider(mnemonic, `https://rinkeby.infura.io/${infuraKey}`),
        network_id: '*',
        gas: 3500000,
        gasPrice: 5000000000, // 50 gwei, this is very high
      },
      mainnet: {
        provider: new HDWalletProvider(mnemonic, `https://mainnet.infura.io/${infuraKey}`),
        network_id: '*',
        gas: 3500000,
        gasPrice: 10000000000, // 50 gwei, this is very high
      },
   },
};
