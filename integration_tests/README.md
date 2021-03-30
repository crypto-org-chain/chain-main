## Pytest

To run integration tests, just run `pytest` in root directory of repo, We use [pytest](https://docs.pytest.org/) to run
integration tests. pytest discover test cases
[automatically](https://docs.pytest.org/en/3.0.6/goodpractices.html#conventions-for-python-test-discovery). notably the
python source files whose name starts with `test_` is treated as test modules, and the functions in them whose name
starts with `test_` will be executed as test cases.

### Pytest in macOS

To run `pytest` successfully in macOS, you need to run it under the `nix-shell` environment. There are two ways to run it in the `nix-shell`:
- run `nix-shell` directly, you will need to assign the chain-maind binary build path (should be the `PROJECT_ROOT/build`) into the `PATH` env of the nix-shell, execute the `make build`, and then execute `pytest`.
- run `nix-shell ./. -A ci-shell`, let the ci shell tobuild the binary automatically, and then execute `pytest`.
- Also, set environment variable `TMPDIR=/tmp` in the user profile, to avoid the long path issue with the unix socket.

### Fixture

We use [pytest.fixture]() to setup/teardown testnet cluster, fixture can be defined at different scopes, we use a
session scoped fixture for default cluster, we can override it with a module scoped one at test modules which require
special testnet setup.

To use a custom cluster for your test case, just create module scoped fixture in your test module [like
this](https://github.com/crypto-org-chain/chain-main/blob/cb0005fd64250e08e4f758138db6a11fcec71d03/integration_tests/test_slashing.py#L17).
you can put the custom cluster config file into the `integration_tests/configs` directory.

To write test case which depend on the default cluster, one only need to create a test function which accept the
`cluster` parameter, then you get access to the [`ClusterCLI`](../pystarport/pystarport/cluster.py#L38) object
automagically.

### Concurrency

We use [python-xdist](https://pypi.org/project/pytest-xdist/) to execute test cases in multi-processess in parallel, the
implications are:

- no memory sharing between test modules

- session scope fixtures might get setup multiple times in different processes, be aware of system resource conflict
  like tcp port numbers. the default cluster fixture compute `base_port` like this: `(100 + worker_id) * 100`, be aware
of this when choosing `base_port` when overriding the default cluster.

  > [pystarport's port rules](../pystarport/README.md#port-rules)

### Markers

We can use [markers](https://docs.pytest.org/en/stable/example/markers.html) to mark test cases, currently we  use
`slow` to mark test cases that runs slow (like slashing test which need to sleep quit long time to wait
for blockchain events), we select or unselect test cases with markers. For example, passing `-m 'not slow'` to pytest
can skip the slow test cases, useful for development.

### Cluster api

`cluster` is an instance of
[`ClusterCLI`](https://github.com/crypto-org-chain/chain-main/blob/master/pystarport/pystarport/cluster.py#L21), which is used
to interact with the chain. `cluster.supervisor` is used to access the embedded [supervisord](http://supervisord.org/)'s
xmlrpc service([api](http://supervisord.org/api.html)). for example:

```python
# stop the chain-maind process of node2
cluster.supervisor.stopProcess('chainmaind-node2')
# start the chain-maind process of node2
cluster.supervisor.startProcess('chainmaind-node2')

# get address of specified account of node0
cluster.address('community')
# get the validator address of node2
cluster.address('validator', i=2, bech='val')
# call the "chain-maind tx bank send -y"
cluster.transfer('from addr', 'to addr', '1cro', i=2)
```

### Temparary directory

We use the [`tmp_path_factory`](https://docs.pytest.org/en/stable/tmpdir.html#the-tmp-path-factory-fixture) to create
the data directory of test chain, pytest will take care of it's cleanup, particularly:

- it's not cleaned up immediately after test runs, so you can investigate it later in development
- it only keeps the directories of recent 3 test runs

For example, you can find the data directory of default cluster in most recent run here: `/tmp/pytest-of-$(whoami)/pytest-current/chaintestcurrent/`.

## pystarport

The integration tests use [pystarport](../pystarport/README.md) to setup chain environment.
