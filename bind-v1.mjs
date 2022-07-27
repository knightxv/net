const startTime = Date.now();
await $`gomobile.exe bind -target="android/arm64" ./bnrtcv1`;
console.log(
  "[%s] bnrtcv1 build success. take %ds",
  new Date().toLocaleString(),
  (Date.now() - startTime) / 1000
);
