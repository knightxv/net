import { newPeer } from "./peer";

(async () => {
  const p1 = newPeer({
    host: "127.0.0.1",
    port: 19020,
    to: `C6MbP6yWiAi6rfRo8KBWrCv6tVW9q3r5D`,
    send: true,
    recv: false,
  });
  const p2 = newPeer({
    host: "127.0.0.1",
    port: 19888,
    to: `2et8jGb14fAfzQBNgLRHcwVxLd4Pif1mn`,
    send: false,
    recv: true,
  });
  const count = 102400;
  const times = 3;
  let mpsTotal = 0;
  for (let i = 0; i < times; i++) {
    console.log(`running test ${i}`);
    const mps2 = p2.run({ count });
    p1.run({ count });
    const x = await mps2;
    console.log(`mps${i}=${x}`);
    mpsTotal += x;
  }

  console.log(`final mps=${Math.round(mpsTotal / times)}`);
})();
