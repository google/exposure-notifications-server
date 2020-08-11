# Performance Test

This directory contains configs and tests for performance metrics collection,
the data can be visualized at
[Mako.dev](https://mako.dev/benchmark?benchmark_key=4790091207671808)

To run these tests, execute [`./scripts/performance.sh <--dev> <#publishes>` ](../../scripts/performance.sh) from the root fo the project. This will also require the setting of env `GOOGLE_APPLICATION_CREDENTIALS`, if this is needed contact @chaodaiG or @MushuEE.

The `--dev` option will run the [development](#Dev) version of the test.

## Production
This uses the production [banchmark_config](./export_test_benchmark.config) and writes results to a [Mako](https://mako.dev/benchmark?benchmark_key=4790091207671808) instance with the provided key.

## Dev
This uses the development [banchmark_config](./export_test_benchmark.dev.config) and writes results to a [Mako](https://mako.dev/benchmark?benchmark_key=5698752440434688) instance with the provided key. You may want to set the number of Publishes within this test for more keys, set that from the the second argument to [performance.sh](../../scripts/performance.sh). Ex. `./scripts/performance.sh --dev 2000`. \**defaults to 1000 for dev and 100000 for prod*