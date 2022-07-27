import path from "path";
const root = path.join(process.cwd());

export const paths = {
    root,
    binTsc: path.resolve(path.join(root, "node_modules/.bin/tsc.cmd")),
};
export interface Package {
    name: string;
    dirName: string;
    packageJson: any;
    paths: {
        package: string;
        packageJson: string;
        tsConfig: string;
    };
}
export function readProjects() {
    const projects = [
        "buffer-typings",
        "buffer",
        "client-typings",
        "client-api",
        "client",
        "client-node",
        "client-web",
        "dchat-typings",
        "dchat",
        "dchat-web",
        "dchat-node",
        "dweb",
        "dweb-web",
        "dweb-node",
        "service",
        "index",
    ];

    return projects.map((x) => {
        const packagePath = path.join(root, "packages", x);
        const packageJsonPath = path.join(packagePath, "package.json");
        const tsConfigPath = path.join(packagePath, "tsconfig.json");
        const packageJson = require(packageJsonPath);
        return {
            name: packageJson.name,
            dirName: x,
            packageJson,
            paths: {
                package: packagePath,
                packageJson: packageJsonPath,
                tsConfig: tsConfigPath,
            },
        } as Package;
    });
}
