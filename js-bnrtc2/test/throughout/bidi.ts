import { newPeer } from "./peer";

(async () => {
  const p1 = newPeer({
    host: "127.0.0.1",
    port: 19020,
    to: `C6MbP6yWiAi6rfRo8KBWrCv6tVW9q3r5D`,
    send: true,
    recv: true,
  });
  const p2 = newPeer({
    host: "127.0.0.1",
    port: 19888,
    to: `2et8jGb14fAfzQBNgLRHcwVxLd4Pif1mn`,
    send: true,
    recv: true,
  });
  const count = 102400;
  const times = 3;
  let mps1Total = 0;
  let mps2Total = 0;
  for (let i = 0; i < times; i++) {
    console.log(`running test ${i}`);
    const mps1 = p1.run({ count });
    const mps2 = p2.run({ count });
    const x = await mps1;
    const y = await mps2;
    console.log(`mps${i}=${x},${y}`);

    mps1Total += await mps1;
    mps2Total += await mps2;
  }

  console.log(`final mps1=${Math.round(mps1Total / times)}`);
  console.log(`final mps2=${Math.round(mps2Total / times)}`);
})();
