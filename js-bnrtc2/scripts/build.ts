import { build } from "esbuild";
import cpr from "cpr";
import { existsSync } from "fs";
import fs from "fs/promises";
import path from "path";
import cp from "child_process";
import { Package, paths, readProjects } from "./util";
const tsConfig = {
  compilerOptions: {
    composite: true,
    incremental: true,
    target: "es2015",
    module: "commonjs",
    lib: [
      "es5",
      "es2015",
      "es2017",
      "es2018",
      "es2019",
      "es2020",
      "es2015.iterable",
      "esnext.asynciterable",
    ],
    types: [],
    declaration: true,
    declarationMap: true,
    sourceMap: false,
    noEmitOnError: true,
    removeComments: true,
    forceConsistentCasingInFileNames: true,
    importsNotUsedAsValues: "error",
    strict: true,
    strictBindCallApply: true,
    strictNullChecks: true,
    skipLibCheck: true,
    noImplicitAny: true,
    moduleResolution: "node",
    esModuleInterop: true,
    newLine: "lf",
    experimentalDecorators: true,
    emitDecoratorMetadata: true,
    downlevelIteration: true,
    outDir: "./build",
    baseUrl: "./",
  },
  exclude: ["./scripts", "./build"],
};

async function runTsc(tsConfigPath: string, isDev: boolean) {
  return new Promise(async (resolve) => {
    const args = [] as string[];
    if (isDev) {
      args.push("-w");
    }
    args.push(...["-p", tsConfigPath]);
    const io = await cp.spawn(paths.binTsc, args);
    io.stdout.on("data", (data) => {
      console.log(String(data));
    });
    io.on("exit", async () => {
      resolve(null);
    });
  });
}
export async function buildAll(isDev: boolean) {
  const projects = readProjects();
  if (isDev) {
    for (const x of projects) {
      await fs.rm(path.join(x.paths.package, "build"), {
        recursive: true,
        force: true,
      });
    }
  }
  const imports: { [key: string]: string[] } = {};
  const entryPoints = [] as string[];
  (tsConfig.compilerOptions as any).paths = imports;
  projects.map((x) => {
    imports[x.name] = [path.join(x.paths.package, "src")];
    entryPoints.push(path.join(x.paths.package, "src"));
  });
  const tsConfigPath = path.join(paths.root, "tsconfig.json");
  await fs.writeFile(tsConfigPath, JSON.stringify(tsConfig, null, 2));
  await runTsc(tsConfigPath, isDev);
}
export async function buildAllSeperated() {
  const projects = readProjects();
  for (const x of projects) {
    console.log(`buliding ${x.name}`);
    await fs.writeFile(x.paths.tsConfig, JSON.stringify(tsConfig, null, 2));
    await runTsc(x.paths.tsConfig, false);
    await fs.unlink(x.paths.tsConfig);
  }
}
