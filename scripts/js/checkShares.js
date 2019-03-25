const axios = require('axios');

const targetMetric = 'num_shares_verified';

const findValue = data => {
  const metrics = data.split('\n').map(d => d.split(' '));
  const metric = metrics.filter(l => !l.includes('#')).find(x => x[0] == targetMetric);
  return metric ? metric[1] : 'metric not found';
};

const INTERVAL_TIME = 1 * 1000;

let nodeStatus = {
  node_one: null,
  node_two: null,
  node_three: null,
  node_four: null,
  node_five: null,
};

let nodeAddress = {
  node_one: 'http://localhost:7001',
  node_two: 'http://localhost:7002',
  node_three: 'http://localhost:7003',
  node_four: 'http://localhost:7004',
  node_five: 'http://localhost:7005',
};

const checkStatus = async () => {
  for (const node in nodeAddress) {
    const nodeAddr = nodeAddress[node];
    axios
      .get(`${nodeAddr}/metrics`)
      .then(resp => {
        if (resp.status < 300) {
          const { data } = resp;
          if (data.includes(targetMetric)) {
            const val = findValue(data);
            nodeStatus[node] = 'shares verified: ' + val;
          } else {
            nodeStatus[node] = 'running';
          }
        }
      })
      .catch(() => {
        // Handle errors
        // not necessary though
      });
  }
};

const printStatus = () => {
  console.clear();
  console.table(nodeStatus);
};

const run = () => {
  setInterval(async () => {
    checkStatus();
    printStatus();
  }, INTERVAL_TIME);
};

run();
