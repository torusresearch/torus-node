const axios = require('axios');

const targetMetrics = [
"num_refreshed",
"processed_messages",
"ignored_message",
"not_dealer",
"not_player",
"already_received_ready",
"invalid_ready",
"invalid_echo",
"sending_ready",
"sending_echo",
"ready_sig_invalid",
"ready_before_echo",
"sending_complete",
"insufficient_recovers",
"invalid_share_commitment",
"sending_proposal",
"invalid_method",
"processed_broadcast_messages",
"decided_sharings_not_complete",
"pssid_uninitialized",
"pssid_incomplete",
"final_shares_invalid",
]


const findValue = (data, targetMetric) => {
  const metrics = data.split('\n').map(d => d.split(' '));
  const metric = metrics.filter(l => !l.includes('#')).find(x => x[0] === targetMetric)
  return metric ? metric[1] : 'metric not found';
};

const INTERVAL_TIME = 1 * 1000;

let nodeStatus = {
  node_six: {},
  node_seven: {},
  node_eight: {},
  node_nine: {},
  node_ten: {},
  node_eleven: {},
  node_twelve: {},
  node_thirteen: {},
  node_fourteen: {},
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
          targetMetrics.map(targetMetric => {
            if (data.includes(targetMetric)) {
              const val = findValue(data, targetMetric);
              nodeStatus[node][targetMetric] = targetMetric + ": " + val;
            } else {
              nodeStatus[node][targetMetric] = 'running';
            }
          })
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

function objToTable(obj) {
  let res = []
}

const run = () => {
  setInterval(async () => {
    checkStatus();
    printStatus();
  }, INTERVAL_TIME);
};

run();
