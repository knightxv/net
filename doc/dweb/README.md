# DWEB

通过`dwebResolve`函数来生成可在地址栏访问的url

> 因为地址栏的限制，二级域名仅支持小写和数字且不得大于62字节，所以在生成url的时候，会把address变成转换成hex并用`.`拆开

```typescript
import { dwebResolve } from "@bfchain/bnrtc2-dweb";
(async () => {
  const url = await dwebResolve(
    "dweb",
    "2et8jGb14fAfzQBNgLRHcwVxLd4Pif1mn",
    19333,
  );
  console.log(url);
  // http://dweb.326574386a476231346641667a51424e674c5248637756784c643450696631.6d6e.localhost:19333
})();
```

通过`DwebServer`来创建服务

> 接口方式参照 express的写法。

```typescript
/// nodejs
import { DwebServer } from "@bfchain/bnrtc2-dweb-node";
/// web
// import { DwebServer } from "@bfchain/bnrtc2-dweb-web";
const dwebServer = new DwebServer();
dwebServer.get("/", (req, res) => {
  res.setAssertType("html");
  res.end('<div style="color: red">node js web server</div>');
});
/// 监听的端口是字符串格式
dwebServer.listen("dweb");
```

通过`use`方法扩展插件(以static插件为例)

```typescript
import {
  StaticFileReader,
  StaticOption,
  serverStatic,
} from "@bfchain/bnrtc2-dweb";
import { DwebServer } from "@bfchain/bnrtc2-dweb-node";
const dwebServer = new DwebServer();
class NodeJsStaticFileReader implements StaticFileReader {
  async readFile(root: string, relativeFile: string): Promise<Uint8Array> {
    const fileName = path.join(root, relativeFile);
    const data = await fs.readFile(fileName);
    return new Uint8Array(data);
  }
}
function nodejsStatic(
  relativeFileDir: string,
  options?: StaticOption
): IDwebHandler {
  const fileReader = new NodeJsStaticFileReader();
  return serverStatic(fileReader, relativeFileDir, options);
}
dwebServer.use(nodejsStatic(path.join(__dirname, "./dist")));
dwebServer.listen("dweb");
```

### 运行demo

在项目根目录通过 `go build`生成 `bnrtc.exe`,分别运行以下命令开启两个节点，并实现互连

```bash
 // 节点1（2et8jGb14fAfzQBNgLRHcwVxLd4Pif1mn）
 .\bnrtc.exe -configFile="./conf/config.json"
 // 节点2（C6MbP6yWiAi6rfRo8KBWrCv6tVW9q3r5D）
 .\bnrtc.exe -configFile="./conf/config2.json"
```

#### 开启节点之后，利用`节点1`来提供dweb服务:

在 `js-bnrtc2/packages/test/server`底下有两个文件夹，`nodejs`以及`web`分别是利用nodejs和web浏览器提供dweb服务。

##### 创建http静态资源服务，在`js-bnrtc2/packages/test/server`运行

```bash
运行
yarn dweb:node
启动nodejs，利用fs读取静态资源提供dweb服务（前提全局命令有ts-node）并且在
`nodejs/dist`要有前端静态资源文件（html,css,js之类的）

或

运行
yarn dweb:web
 利用浏览器 fileReader 提供静态资源
（前提要安装好 parcel 全局命令，启动之后浏览器打开`localhost:1234`，点击上传文件
，选择文件夹之后，点击页面上的`上传按钮`即可提供静态资源服务）
```

##### 开启服务之后就通过节点2来访问静态资源。

在项目的`js-bnrtc2/packages/test/server`有个getUrl.ts文件，可以运行

```bash
ts-node ./getUrl.ts
```

并打印出相对应的地址。（要根据具体的开启的dport端口以及dweb端口开调整参数）

然后粘贴至浏览器访问
