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
       port: 8545,
       network_id: '*', // Match any network id
       gas: 90000000,
     },
    ropsten: {
       provider: new HDWalletProvider(mnemonic, `https://ropsten.infura.io/${infuraKey}`),
       network_id: '*',
       gas: 4700000,
       gasPrice: 3000000000, // 50 gwei, this is very high
     },
     rinkeby: {
        provider: new HDWalletProvider(mnemonic, `https://rinkeby.infura.io/${infuraKey}`),
        network_id: '*',
        gas: 3500000,
        gasPrice: 5000000000, // 50 gwei, this is very high
      },
   },
};
