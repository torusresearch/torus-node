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
const HDWalletProvider = require("@truffle/hdwallet-provider");
// const private = require('../private.json');
// const mnemonic = private.funds;
// const infuraKey = private.infura;
const mnemonic = "oil foam cement clerk open rough entry swarm poverty length tail portion";
module.exports = {
  // See <http://truffleframework.com/docs/advanced/configuration>
  // to customize your Truffle configuration!
  networks: {
    development: {
      host: "127.0.0.1",
      port: 7545,
      network_id: "*"
    },
    coverage: {
      host: "localhost",
      port: 8555, // <-- Use port 8555
      gas: 0xfffffffffff, // <-- Use this high gas value
      gasPrice: 0x01, // <-- Use this low gas price
      network_id: "*"
    },
    mainscript: {
      host: "localhost",
      port: 14103,
      network_id: "*", // Match any network id
      gas: 4700000
    },
    staging: {
      network_id: "*", // Match any network id
      provider: function() {
        return new HDWalletProvider(mnemonic, "https://ganache.staging.dev.tor.us");
      },
      gas: 4700000
    }
    // digital: {
    //   provider: new HDWalletProvider(private.ganache, 'http://178.128.178.162:14103'),
    //   network_id: '*',
    //   network_id: '*',
    //   gas: 4700000,
    //   gasPrice: 5000000000, // 50 gwei, this is very high
    // },
    // ropsten: {
    //   provider: new HDWalletProvider(mnemonic, `https://ropsten.infura.io/${infuraKey}`),
    //   network_id: '*',
    //   gas: 4700000,
    //   gasPrice: 5000000000, // 50 gwei, this is very high
    // },
    // rinkeby: {
    //   provider: new HDWalletProvider(mnemonic, `https://rinkeby.infura.io/${infuraKey}`),
    //   network_id: '*',
    //   gas: 3500000,
    //   gasPrice: 5000000000, // 50 gwei, this is very high
    // },
    // mainnet: {
    //   provider: new HDWalletProvider(mnemonic, `https://mainnet.infura.io/${infuraKey}`),
    //   network_id: '*',
    //   gas: 3500000,
    //   gasPrice: 10000000000, // 50 gwei, this is very high
    // },
  },
  mocha: {
    timeout: 100000
  },
  // Configure your compilers
  compilers: {
    solc: {
      version: "0.5.15", // Fetch exact version from solc-bin (default: truffle's version)
      // docker: true,        // Use "0.5.1" you've installed locally with docker (default: false)
      settings: {
        // See the solidity docs for advice about optimization and evmVersion
        optimizer: {
          enabled: true,
          runs: 600
        }
      }
    }
  },
  plugins: ["solidity-coverage"]
};
