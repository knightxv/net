declare namespace Bnrtc2 {
    interface Bnrtc2Buffer {
        length: number;
        capacity: number;
        buffer: Readonly<Uint8Array>;
        // 往buffer头部写入
        put(data: Uint8Array): Bnrtc2Buffer;
        putBool(v: boolean): Bnrtc2Buffer;
        putU8(v: number): Bnrtc2Buffer;
        putU16(v: number): Bnrtc2Buffer;
        putU32(v: number): Bnrtc2Buffer;
        putStr(str: string): Bnrtc2Buffer;
        // 往buffer尾部写入
        push(data: Uint8Array): Bnrtc2Buffer;
        pushBool(v: boolean): Bnrtc2Buffer;
        pushU8(v: number): Bnrtc2Buffer;
        pushU16(v: number): Bnrtc2Buffer;
        pushU32(v: number): Bnrtc2Buffer;
        pushStr(str: string): Bnrtc2Buffer;
        // 从buffer头部读取
        pull(): Uint8Array;
        pullBool(): boolean;
        pullU8(): number;
        pullU16(): number;
        pullU32(): number;
        pullStr(): string;
        // 从buffer头部窥视
        peek(): Uint8Array;
        peekBool(): boolean;
        peekU8(): number;
        peekU16(): number;
        peekU32(): number;
        peekStr(): string;
        // 克隆buffer，公用底层数据，只读
        clone(): Bnrtc2Buffer;
        // 获取数据dataView
        data(): Readonly<Uint8Array>;
        // 复制buffer，底层独立，互不影响
        copy(): Bnrtc2Buffer;
    }

    interface Bnrtc2BufferConstructor {
        // 将Uint8Array转成Bnrtc2Buffer，会写时拷贝
        from(data: Uint8Array): Bnrtc2Buffer;
        // size为预留空间大小，data存在则会立即拷贝一份
        create(size: number, data?: Uint8Array): Bnrtc2Buffer;
    }
}