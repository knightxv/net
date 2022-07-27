import yargs from "yargs";
import { hideBin } from "yargs/helpers";
import { buildAll, buildAllSeperated } from "./build";
import { publish } from "./publish";

yargs(hideBin(process.argv))
  .command(
    "build",
    "build all packages",
    (yargs) => yargs,
    (argv) => {
      buildAllSeperated();
    }
  )
  .command(
    "dev",
    "build all packages in watch mode",
    (yargs) => yargs,
    (argv) => {
      buildAll(true);
    }
  )
  .command(
    "publish",
    "publish packages to npm",
    (yargs) => {
      return yargs.positional("v", {});
    },
    (argv) => {
      if (!argv.v) {
        console.warn("missing args --v");
        return;
      }
      publish(argv.v as string);
    }
  )

  .parse();
