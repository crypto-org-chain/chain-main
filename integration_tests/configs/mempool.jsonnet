local default = import 'default.jsonnet';

{
  'mempool-test': default.chaintest {
    config: {
      mempool: {
        version: 'v1',
      },
      consensus: {
        timeout_commit: '5s',
      },
    },
  },
}
