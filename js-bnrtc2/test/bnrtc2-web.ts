import "./bnrtc2-web.shim";
import { Bnrtc2Client } from "@bfchain/bnrtc2-client-web";
import { Dchat } from "@bfchain/bnrtc2-dchat-web";

declare global {
  interface Window {
    Bnrtc2Client: typeof Bnrtc2Client;
    Dchat: typeof Dchat;
  }
}

window.Bnrtc2Client = Bnrtc2Client;
window.Dchat = Dchat;
