const child_process=require('child_process');
const fs=require('fs');

let num = 20;
let startHttpPort = 10000;
let startServicePort = 20000;
let startDebugPort = 25000;

let peers = ["127.0.0.1:20000"];


for (let i = 0; i < num; i++) {

    let address = "batchTest"+i;
    let configFile = address+".json";
    let config ={
      name:              address,
      host:              "0.0.0.0",
      port:              startHttpPort + i,
      servicePort:       startServicePort + i,
      debugPort:         startDebugPort + i,
      forceTopNetwork: false,
      addresses:         [address],
      peers:             peers,
      disableTurnServer: true,
      disableStunServer: true,
      disableDweb:       true,
      disableDebug:      false,
    }
    if (i == 0) {
        config.forceTopNetwork = true;
        config.services = {
          dchat: {
              enabled: true
          }
        }
    }


    fs.writeFileSync(configFile, JSON.stringify(config, "", "\t"));
    const bnrtc = child_process.spawn('./bnrtc', ["--config=" + configFile, "--name=" + address]);

    bnrtc.stdout.on('data', (data) => {
      console.log(`stdout: ${data}`);
    });

    bnrtc.stderr.on('data', (data) => {
      console.log(`stderr: ${data}`);
    });

    bnrtc.on('close', (code) => {
      console.log(`child process exited with code ${code}`);
    });
}