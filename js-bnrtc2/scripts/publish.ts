import execa from "execa";
import { build } from "esbuild";
import cpr from "cpr";
import path from "path";
import fs from "fs/promises";
import { existsSync } from "fs";
import { Package, paths, readProjects } from "./util";
import { buildAll } from "./build";

export async function publish(version: string) {
  const dstPathBuildRoot = path.join(paths.root, "build");
  if (!existsSync(dstPathBuildRoot)) {
    await fs.mkdir(dstPathBuildRoot);
  }

  const projects = readProjects();
  const projectNames = projects.map((x) => x.name);
  for (const x of projects) {
    await updateVersion(x, version, projectNames);
    await copyPackage(x, dstPathBuildRoot, false);
  }
  console.log(
    `generated packages (${version}) to be published at ${path.join(
      dstPathBuildRoot,
      "publish"
    )}`
  );
  for (const x of projects) {
    const publishRoot = path.join(dstPathBuildRoot, "publish", x.name);
    const { stdout, stderr } = await execa(`npm`, [
      "publish",
      publishRoot,
      "--registry=https://registry.npmjs.com",
      // "--registry=http://localhost:4873",
      "--access=public",
    ]);
    console.log(stdout);
    // console.log(stderr);
  }
}
async function updateVersion(
  x: Package,
  version: string,
  packageNames: string[]
) {
  x.packageJson.version = version;
  Object.keys(x.packageJson.dependencies).forEach((k) => {
    if (packageNames.some((p) => p === k)) {
      x.packageJson.dependencies[k] = `^${version}`;
    }
  });
  await fs.writeFile(
    x.paths.packageJson,
    JSON.stringify(x.packageJson, null, 2)
  );
}

async function copyPackage(
  x: Package,
  dstPathBuildRoot: string,
  removeSource: boolean
) {
  const publishRoot = path.join(dstPathBuildRoot, "publish", x.name);
  const srcPathBuild = path.join(x.paths.package, "build", "src");
  const dstPathBuild = path.join(publishRoot, "build");
  if (!existsSync(dstPathBuild)) {
    await fs.mkdir(dstPathBuild, { recursive: true });
  }

  // await build({
  //   entryPoints: [path.join(srcPathBuild, "index.js")],
  //   format: "cjs",
  //   bundle: true,
  //   minify: true,
  //   plugins: [
  //     {
  //       name: "non-path-are-external",
  //       setup(build) {
  //         build.onResolve({ filter: /^[^\.].*/ }, async (args) => {
  //           if (path.isAbsolute(args.path)) {
  //             // console.log(`${args.path} is abs`);
  //             return { path: args.path };
  //           }
  //           return {
  //             external: true,
  //           };
  //         });
  //       },
  //     },
  //     // {
  //     //   name: "platform-selection",
  //     //   setup(build) {
  //     //     build.onResolve({ filter: /^\..*#$/ }, async (args) => {
  //     //       // console.log("resolving",args.path)
  //     //       const p = path.join(args.resolveDir, `${args.path}${platform}.js`);
  //     //       if (existsSync(p)) {
  //     //         console.log(`selected platform ${platform} for ${args.path}`);
  //     //         return { path: p };
  //     //       }
  //     //       return { path: path.join(args.resolveDir, `${args.path}.js`) };
  //     //     });
  //     //   },
  //     // },
  //   ],
  //   outfile: path.join(dstPathBuild, "index.js"),
  // });

  const copyTask = new Promise((resolve) => {
    cpr(
      srcPathBuild,
      dstPathBuild,
      { overwrite: true, filter: /\.map$/ },
      async (err, files) => {
        err && console.log(err);
        resolve(null);
      }
    );
  });
  await copyTask;
  if (removeSource) {
    await fs.rm(srcPathBuild, {
      recursive: true,
      force: true,
    });
  }

  const packageJson = Object.assign({}, x.packageJson);
  packageJson.main = `./build/index.js`;
  packageJson.types = `./build/index.d.ts`;
  packageJson.license = "CC-BY-NC-SA-4.0";
  packageJson.files = ["build"];
  packageJson.peerDependencies = {};
  delete packageJson.scripts;
  const transformed = [] as string[];
  Object.keys(packageJson.dependencies).forEach((x) => {
    if (x.startsWith("@bfchain/util")) {
      transformed.push(x);
      packageJson.peerDependencies[x] = packageJson.dependencies[x];
      delete packageJson.dependencies[x];
    }
  });
  if (transformed.length > 0) {
    console.group(`${packageJson.name} peerDependencies`);
    transformed.map((x) => console.log(`${x}`));
    console.log(`\n`);
    console.groupEnd();
  }

  await fs.writeFile(
    path.join(publishRoot, "package.json"),
    JSON.stringify(packageJson, null, 2)
  );
}
