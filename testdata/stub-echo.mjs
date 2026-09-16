// Stub CLI used by clawshim integration tests. Prints a JSON summary of
// argv/env so tests can assert exact argument round-trips through the shim.
import { createHash } from 'node:crypto';
import { appendFileSync } from 'node:fs';

const argv = process.argv.slice(2);

const countFile = process.env.CLAWSHIM_TEST_COUNT_FILE;
if (countFile) {
  appendFileSync(countFile, 'run\n');
}

if (argv.length === 1 && argv[0] === '--version') {
  console.log('OpenClaw 2099.1.2 (stub)');
} else {
  const summary = {
    argc: argv.length,
    maxArgLen: argv.reduce((m, a) => Math.max(m, a.length), 0),
    totalLen: argv.reduce((m, a) => m + a.length, 0),
    env: process.env.CLAWSHIM_TEST_ENV ?? null,
    sha1: createHash('sha1').update(argv.join('\u0000')).digest('hex'),
  };
  console.log(JSON.stringify(summary));
}

const exitArg = argv.find((a) => a.startsWith('exit:'));
if (exitArg) {
  process.exit(Number.parseInt(exitArg.slice(5), 10) || 0);
}
