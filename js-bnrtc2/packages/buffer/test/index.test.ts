import { Bnrtc2Buffer } from "../src/index";
import test from "ava";

test("from/length/capacity", (t) => {
  const len = 10;
  const data = new Uint8Array(len).fill(1);
  const buf = Bnrtc2Buffer.from(data);
  t.is(buf.length, len);
  t.is(buf.capacity, len);
  buf.putU8(2);
  t.is(buf.length, len + 1);
  t.is(buf.capacity, 256);
  t.is(buf.pullU8(), 2);
});

test("create/len/capacity", (t) => {
  const len = 10;
  const data = new Uint8Array(len).fill(1);
  const buf = Bnrtc2Buffer.create(len, data);
  t.is(buf.length, len);
  t.is(buf.capacity, 256);
  buf.putU8(1);
  t.is(buf.length, len + 1);
  t.is(buf.capacity, 256);
});

test("put/push/pull/peek", (t) => {
  const len = 10;
  const data = new Uint8Array(len).fill(1);
  const buf = Bnrtc2Buffer.create(0);
  buf.put(data);
  t.deepEqual(buf.peek(), data);
  t.deepEqual(buf.pull(), data);
  t.is(buf.length, 0);

  buf.push(data);
  t.deepEqual(buf.peek(), data);
  t.deepEqual(buf.pull(), data);
  t.is(buf.length, 0);
});

test("putU8/pushU8/pullU8/peekU8", (t) => {
  const buf = Bnrtc2Buffer.create(0);
  const testPut = (buf: Bnrtc2Buffer, value: number, e: number) => {
    buf.putU8(value);
    t.is(buf.length, 1);
    t.is(buf.peekU8(), e);
    t.is(buf.length, 1);
    const d = buf.pullU8();
    t.is(d, e);
    t.is(buf.length, 0);
  };
  testPut(buf, 0, 0);
  testPut(buf, 1, 1);
  testPut(buf, 0xff, 0xff);
  testPut(buf, 0xff + 1, (0xff + 1) & 0xff);
  testPut(buf, -1, -1 & 0xff);
  const v1 = Math.floor(Math.random() * 0xff);
  testPut(buf, v1, v1);

  const testPush = (buf: Bnrtc2Buffer, value: number, e: number) => {
    buf.pushU8(value);
    t.is(buf.length, 1);
    t.is(buf.peekU8(), e);
    t.is(buf.length, 1);
    const d = buf.pullU8();
    t.is(d, e);
    t.is(buf.length, 0);
  };
  testPush(buf, 0, 0);
  testPush(buf, 1, 1);
  testPush(buf, 0xff, 0xff);
  testPush(buf, 0xff + 1, (0xff + 1) & 0xff);
  testPush(buf, -1, -1 & 0xff);
  const v2 = Math.floor(Math.random() * 0xff);
  testPush(buf, v2, v2);
});

test("putU16/pushU16/pullU16/peekU16", (t) => {
  const buf = Bnrtc2Buffer.create(0);
  const testPut = (buf: Bnrtc2Buffer, value: number, e: number) => {
    buf.putU16(value);
    t.is(buf.length, 2);
    t.is(buf.peekU16(), e);
    t.is(buf.length, 2);
    const d = buf.pullU16();
    t.is(d, e);
    t.is(buf.length, 0);
  };
  testPut(buf, 0, 0);
  testPut(buf, 1, 1);
  testPut(buf, 0xffff, 0xffff);
  testPut(buf, 0xffff + 1, (0xffff + 1) & 0xffff);
  testPut(buf, -1, -1 & 0xffff);
  const v1 = Math.floor(Math.random() * 0xffff);
  testPut(buf, v1, v1);

  const testPush = (buf: Bnrtc2Buffer, value: number, e: number) => {
    buf.pushU16(value);
    t.is(buf.length, 2);
    t.is(buf.peekU16(), e);
    t.is(buf.length, 2);
    const d = buf.pullU16();
    t.is(d, e);
    t.is(buf.length, 0);
  };
  testPush(buf, 0, 0);
  testPush(buf, 1, 1);
  testPush(buf, 0xffff, 0xffff);
  testPush(buf, 0xffff + 1, (0xffff + 1) & 0xffff);
  testPush(buf, -1, -1 & 0xffff);
  const v2 = Math.floor(Math.random() * 0xffff);
  testPush(buf, v2, v2);
});

test("putU23/pushU32/pullU32/peekU32", (t) => {
  const buf = Bnrtc2Buffer.create(0);
  const testPut = (buf: Bnrtc2Buffer, value: number, e: number) => {
    buf.putU32(value);
    t.is(buf.length, 4);
    t.is(buf.peekU32(), e);
    t.is(buf.length, 4);
    const d = buf.pullU32();
    t.is(d, e);
    t.is(buf.length, 0);
  };
  testPut(buf, 0, 0);
  testPut(buf, 1, 1);
  testPut(buf, 0xffffffff, 0xffffffff);
  testPut(buf, 0xffffffff + 1, (0xffffffff + 1) & 0xffffffff);
  testPut(buf, -1, -1 >>> 0);
  const v1 = Math.floor(Math.random() * 0xffffffff);
  testPut(buf, v1, v1);

  const testPush = (buf: Bnrtc2Buffer, value: number, e: number) => {
    buf.pushU32(value);
    t.is(buf.length, 4);
    t.is(buf.peekU32(), e);
    t.is(buf.length, 4);
    const d = buf.pullU32();
    t.is(d, e);
    t.is(buf.length, 0);
  };
  testPush(buf, 0, 0);
  testPush(buf, 1, 1);
  testPush(buf, 0xffffffff, 0xffffffff);
  testPush(buf, 0xffffffff + 1, (0xffffffff + 1) & 0xffffffff);
  testPush(buf, -1, -1 >>> 0);
  const v2 = Math.floor(Math.random() * 0xffffffff);
  testPush(buf, v2, v2);
});

test("putStr/pushStr/pullStr/peekStr", (t) => {
  const buf = Bnrtc2Buffer.create(0);
  const str = "I am test, 我是测试";
  let data: string;
  buf.putStr(str);
  t.is(buf.peekStr(), str);
  data = buf.pullStr();
  t.is(buf.length, 0);
  t.is(data, str);

  buf.pushStr(str);
  t.is(buf.peekStr(), str);
  data = buf.pullStr();
  t.is(buf.length, 0);
  t.is(data, str);
});

test("put/push/pull Times", (t) => {
  const buf = Bnrtc2Buffer.create(0);
  const dataLen = 10;
  const data1 = new Uint8Array(dataLen).fill(1);
  const data2 = new Uint8Array(dataLen).fill(2);
  const str1 = "I am test";
  const str2 = "I am test2";
  const u8_1 = 10;
  const u8_2 = 100;
  const u16_1 = 0xff + 10;
  const u16_2 = 0xff + 100;
  const u32_1 = 0xffff + 10;
  const u32_2 = 0xffff + 100;
  let strLen1: number;
  let strLen2: number;

  buf.put(data1);
  buf.push(data2);
  buf.putStr(str1);
  buf.pushStr(str2);
  buf.putU8(u8_1);
  buf.pushU8(u8_2);
  buf.putU16(u16_1);
  buf.pushU16(u16_2);
  buf.putU32(u32_1);
  buf.pushU32(u32_2);

  t.is(buf.pullU32(), u32_1);
  t.is(buf.pullU16(), u16_1);
  t.is(buf.pullU8(), u8_1);
  t.is(buf.pullStr(), str1);
  t.deepEqual(buf.pull(), data1);
  t.deepEqual(buf.pull(), data2);
  t.is(buf.pullStr(), str2);
  t.is(buf.pullU8(), u8_2);
  t.is(buf.pullU16(), u16_2);
  t.is(buf.pullU32(), u32_2);
  t.is(buf.length, 0);
});

test("clone", (t) => {
  const buf = Bnrtc2Buffer.create(0);
  buf.putU8(1);
  const buf2 = buf.clone();
  t.is(buf.buffer === buf2.buffer, true);

  let error = t.throws(
    () => {
      buf2.pushU8(1);
    },
    { instanceOf: RangeError }
  );
  t.true(error instanceof RangeError);

  error = t.throws(
    () => {
      buf2.putU8(1);
    },
    { instanceOf: RangeError }
  );
  t.true(error instanceof RangeError);

  buf.putU8(1);
  buf.pullU8();

  buf.putU8(2);
  t.is(buf.buffer === buf2.buffer, false);

  buf.pushU8(3);
  t.is(buf.pullU8(), 2);
  t.is(buf.pullU8(), 1);
  t.is(buf.pullU8(), 3);
  error = t.throws(
    () => {
      buf.pullU8();
    },
    { instanceOf: RangeError }
  );
  t.true(error instanceof RangeError);

  t.is(buf2.pullU8(), 1);
  error = t.throws(
    () => {
      buf2.pullU8();
    },
    { instanceOf: RangeError }
  );
  t.true(error instanceof RangeError);
});

test("copy", (t) => {
  const buf = Bnrtc2Buffer.create(0);
  buf.putU8(1);
  const buf2 = buf.copy();
  t.is(buf.buffer === buf2.buffer, false);

  buf.putU8(2);
  buf.pushU8(3);
  buf2.putU8(4);
  buf2.pushU8(5);
  t.is(buf.pullU8(), 2);
  t.is(buf.pullU8(), 1);
  t.is(buf.pullU8(), 3);
  let error = t.throws(
    () => {
      buf.pullU8();
    },
    { instanceOf: RangeError }
  );
  t.true(error instanceof RangeError);

  t.is(buf2.pullU8(), 4);
  t.is(buf2.pullU8(), 1);
  t.is(buf2.pullU8(), 5);
  error = t.throws(
    () => {
      buf2.pullU8();
    },
    { instanceOf: RangeError }
  );
  t.true(error instanceof RangeError);
});

test("all", (t) => {
  const u8 = 10;
  const u16 = 65533;
  const u32 = 65566;
  const str =
    "我不是我，布拉all哆啦我来顶啦我来到了dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddwdadawdawdawdfergwtreh";
  const dataLen = 300;
  const data = new Uint8Array(dataLen).fill(1);

  const buf = Bnrtc2Buffer.create(0);
  buf.pushStr(str);
  buf.put(data);
  buf.putU8(u8);
  buf.putU16(u16);
  buf.putU32(u32);
  buf.pushStr(str);
  buf.push(data);
  buf.pushU8(u8);
  buf.pushU16(u16);
  buf.pushU32(u32);

  t.is(buf.pullU32(), u32);
  t.is(buf.pullU16(), u16);
  t.is(buf.pullU8(), u8);
  t.deepEqual(buf.pull(), data);
  t.deepEqual(buf.pullStr(), str);
  t.deepEqual(buf.pullStr(), str);
  t.deepEqual(buf.pull(), data);
  t.is(buf.pullU8(), u8);
  t.is(buf.pullU16(), u16);
  t.is(buf.pullU32(), u32);
});
