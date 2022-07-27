import "@bfchain/bnrtc2-buffer-typings";
import { encodeUTF8ToBinary, decodeBinaryToUTF8 } from "@bfchain/util-encoding-utf8";

function nextpow2(n: number): number {
    if (n >= Math.pow(2, 32)) {
        return n;
    }

    n = n - 1; // 这个操作决定当n刚好是2的次幂值时，取n，如果没有，则取2n
    n |= n >>> 1;
    n |= n >>> 2;
    n |= n >>> 4;
    n |= n >>> 8;
    n |= n >>> 16;
    n += 1;
    return n;
}

enum WriteDirection {
    PUT = -1,
    PUSH = 1,
}

const MAX_DATA_LEN = Math.pow(2, 32)

export class Bnrtc2Buffer implements Bnrtc2.Bnrtc2Buffer {
    private static HEAD_RESERVE_SIZE: number = 128;
    static from(data: Uint8Array): Bnrtc2Buffer {
        return new Bnrtc2Buffer(data);
    }
    static create(size: number, data?: Uint8Array): Bnrtc2Buffer {
        if (size < 0) {
            throw RangeError(`size(${size}) must > 0`);
        }

        if (data && size < data.length) {
            size = data.length;
        }

        const len = nextpow2(size + Bnrtc2Buffer.HEAD_RESERVE_SIZE);
        const u8a = new Uint8Array(len);
        const buf = new Bnrtc2Buffer(u8a, Bnrtc2Buffer.HEAD_RESERVE_SIZE, Bnrtc2Buffer.HEAD_RESERVE_SIZE);
        if (data) {
            buf._buf.set(data, buf._endOffset);
            buf._endOffset += size;
        }
        return buf;
    }

    private _buf: Uint8Array;
    private _dataOffset: number
    private _endOffset: number;
    private _isPulled: boolean = false;
    private _isClone: boolean = false;
    private _hasCloned: boolean = false;

    private constructor(data: Uint8Array, satrt: number = 0, end: number = data.length) {
        this._buf = data;
        this._dataOffset = satrt;
        this._endOffset = end;
    }

    private extendBuffer(size: number, direction: WriteDirection) {
        const oldData = this.data()
        const oldDataLen = this.length
        const len = nextpow2(oldDataLen + size + Bnrtc2Buffer.HEAD_RESERVE_SIZE);
        this._buf = new Uint8Array(len);
        if (direction == WriteDirection.PUSH) {
            this._dataOffset = Bnrtc2Buffer.HEAD_RESERVE_SIZE;
            this._buf.set(oldData, this._dataOffset);
            this._endOffset = Bnrtc2Buffer.HEAD_RESERVE_SIZE + oldDataLen;
        } else {
            this._dataOffset = Bnrtc2Buffer.HEAD_RESERVE_SIZE + size;
            this._buf.set(oldData, this._dataOffset);
            this._endOffset = Bnrtc2Buffer.HEAD_RESERVE_SIZE + size + oldDataLen;
        }
        this._isClone = false;
        this._hasCloned = false;
    }


    private checkSpace(len: number, direction: WriteDirection): Bnrtc2Buffer {
        if (len + this.length >= MAX_DATA_LEN) {
            throw new RangeError(`data length(${len + this.length}} exceed limit(${MAX_DATA_LEN})`);
        }

        if (this._isClone) {
            throw new RangeError(`clone buffer is read only`);
        }


        if (direction == WriteDirection.PUT) {
            if (len > this._dataOffset || (this._hasCloned && this._isPulled)) {
                this.extendBuffer(len, direction);
            }
        } else if (direction == WriteDirection.PUSH) {
            if (this._endOffset + len > this._buf.length) {
                this.extendBuffer(len, direction);
            }
        }
        return this;
    }

    get length() {
        return (this._endOffset  - this._dataOffset);
    }

    get capacity() {
        return this._buf.length;
    }

    get buffer() {
        return this._buf;
    }
    // 往前插入
    private _put(data: Uint8Array): Bnrtc2Buffer {
        const len = data.length;
        const buf = this.checkSpace(len, WriteDirection.PUT);
        buf._dataOffset -= len;
        buf._buf.set(data, this._dataOffset);
        return buf;
    }

    public put(data: Uint8Array): Bnrtc2Buffer {
        return this._put(data).putU32(data.length)
    }
    public putBool(value: boolean): Bnrtc2Buffer {
        return this.putU8(value ? 1 : 0)
    }
    public putU8(value: number): Bnrtc2Buffer {
        value = value & 0XFF
        const buf = this.checkSpace(1, WriteDirection.PUT);
        buf._dataOffset -= 1;
        buf._buf[this._dataOffset] = value;
        return buf;
    }
    public putU16(value: number): Bnrtc2Buffer {
        value = value & 0XFFFF
        const buf = this.checkSpace(2, WriteDirection.PUT);
        buf._dataOffset -= 2;
        buf._buf[this._dataOffset] = value >> 8;
        buf._buf[this._dataOffset+1] = value;
        return buf;
    }
    public putU32(value: number): Bnrtc2Buffer {
        value >>>= 0
        const buf = this.checkSpace(4, WriteDirection.PUT);
        buf._dataOffset -= 4;
        buf._buf[this._dataOffset] = value >> 24;
        buf._buf[this._dataOffset+1] = value >> 16;
        buf._buf[this._dataOffset+2] = value >> 8;
        buf._buf[this._dataOffset+3] = value;
        return buf;
    }
    public putStr(str: string): Bnrtc2Buffer {
        const u8a = encodeUTF8ToBinary(str);
        return this.put(u8a);
    }

    private _push(data: Uint8Array): Bnrtc2Buffer {
        const len = data.length;
        const buf = this.checkSpace(len, WriteDirection.PUSH);
        buf._buf.set(data, this._endOffset);
        buf._endOffset += len;
        return buf;
    }
    public push(data: Uint8Array): Bnrtc2Buffer {
        return this.pushU32(data.length)._push(data);
    }
    public pushBool(value: boolean): Bnrtc2Buffer {
        return this.pushU8(value ? 1 : 0)
    }
    public pushU8(value: number): Bnrtc2Buffer {
        value = value & 0XFF
        const buf = this.checkSpace(1, WriteDirection.PUSH);
        buf._buf[this._endOffset] = value;
        buf._endOffset += 1;
        return buf;
    }
    public pushU16(value: number): Bnrtc2Buffer {
        value = value & 0XFFFF
        const buf = this.checkSpace(2, WriteDirection.PUSH);
        buf._buf[this._endOffset] = value >> 8;
        buf._buf[this._endOffset+1] = value;
        buf._endOffset += 2;

        return buf;
    }
    public pushU32(value: number): Bnrtc2Buffer {
        value >>>= 0
        const buf = this.checkSpace(4, WriteDirection.PUSH);
        buf._buf[this._endOffset] = value >> 24;
        buf._buf[this._endOffset+1] = value >> 16;
        buf._buf[this._endOffset+2] = value >> 8;
        buf._buf[this._endOffset+3] = value;
        buf._endOffset += 4;

        return buf;
    }
    public pushStr(str: string): Bnrtc2Buffer {
        const u8a = encodeUTF8ToBinary(str);
        return this.push(u8a);
    }

    private _pull(size: number): Uint8Array {
        const u8a = this._peek(size)
        this._dataOffset += size;
        this._isPulled = true;
        return u8a;
    }

    public pull(): Uint8Array {
        const len = this.pullU32()
        return this._pull(len)
    }

    public pullBool(): boolean {
        return this.pullU8() != 0
    }

    public pullU8(): number {
        const u8a = this._pull(1);
        return u8a[0];
    }
    public pullU16(): number {
        const u8a = this._pull(2);
        return (u8a[0] << 8) | u8a[1];
    }
    public pullU32(): number {
        const u8a = this._pull(4);
        // 这里如果使用u8a[0] << 24 会变成有符号
        return ((u8a[0] << 24) | (u8a[1] << 16) | (u8a[2] << 8) | u8a[3])>>>0;
    }
    public pullStr(): string {
        const u8a = this.pull()
        return decodeBinaryToUTF8(u8a);
    }

    private _peek(size: number, offset: number = 0): Uint8Array {
        const len = this._endOffset - this._dataOffset - offset;
        if (size > len) {
            throw new RangeError(`size(${size}) > data length(${len})`);
        }

        return this._buf.subarray(this._dataOffset+offset, this._dataOffset+offset+size);
    }
    public peek(): Uint8Array {
        const len = this.peekU32()
        return this._peek(len, 4)
    }
    public peekBool(): boolean {
        return this.peekU8() != 0
    }
    public peekU8(): number {
        const u8a = this._peek(1);
        return u8a[0];
    }
    public peekU16(): number {
        const u8a = this._peek(2);
        return (u8a[0] << 8) | u8a[1];
    }
    public peekU32(): number {
        const u8a = this._peek(4);
        // 这里如果使用u8a[0] << 24 会变成有符号
        return ((u8a[0] << 24) | (u8a[1] << 16) | (u8a[2] << 8) | u8a[3])>>>0;
    }
    public peekStr(): string {
        const u8a = this.peek()
        return decodeBinaryToUTF8(u8a);
    }

    // not copy, just a dataView
    public data(): Uint8Array {
        return this._buf.subarray(this._dataOffset, this._endOffset);
    }

    public clone(): Bnrtc2Buffer {
        const buf = new Bnrtc2Buffer(this._buf, this._dataOffset, this._endOffset);
        buf._isClone = true;
        this._hasCloned = true;
        return buf;
    }

    public copy(): Bnrtc2Buffer {
        return Bnrtc2Buffer.create(this.length, this.data());
    }
}
