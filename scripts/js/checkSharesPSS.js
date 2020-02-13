const axios = require('axios');

const targetMetric = 'num_shares_verified';

const findValue = data => {
  const metrics = data.split('\n').map(d => d.split(' '));
  const metric = metrics.filter(l => !l.includes('#')).find(x => x[0] == targetMetric);
  return metric ? metric[1] : 'metric not found';
};

const INTERVAL_TIME = 1 * 1000;

let nodeStatus = {
  node_six: null,
  node_seven: null,
  node_eight: null,
  node_nine: null,
  node_ten: null,
  node_eleven: null,
  node_twelve: null,
  node_thirteen: null,
  node_fourteen: null,
};

let nodeAddress = {
  node_six: 'http://localhost:7006',
  node_seven: 'http://localhost:7007',
  node_eight: 'http://localhost:7008',
  node_nine: 'http://localhost:7009',
  node_ten: 'http://localhost:7010',
  node_eleven: 'http://localhost:7011',
  node_twelve: 'http://localhost:7012',
  node_thirteen: 'http://localhost:7013',
  node_fourteen: 'http://localhost:7014',
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
