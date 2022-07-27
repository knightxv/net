export const enum READY_STATE {
  CONNECTING,
  OPEN,
  CLOSING,
  CLOSE,
}

export const BNRTC1_PROTOCOL = "bnrtc1:";

export const enum RESULT_CODE {
  SUCCESS = 0,
  FAILED = 1,
  OFFLINE = 2,
  DPORT_CONFLICT = 3,
  DPORT_UNBOUND = 4,
  CONNECT_FAILED = 5,
  FORMAT_ERR = 6,
  TIMEOUT = 7,
}

export const enum MESSAGE_TYPE {
  BINDDPORT = 0,    // for send only
  UNBINDDPORT = 1,  // for send only
  DATA = 2,
  MULTICAST = 3,    // for send only
  BROADCAST = 4,    // for send only
  SYNC = 5,
  ACK = 6,
  NEIGHBORCAST = 7, // for send only
}