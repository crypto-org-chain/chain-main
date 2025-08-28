# VersionDB

VersionDB is a solution for the size issue of IAVL database, aka. `application.db`, at current stage, it's only recommended for archive and non-validator nodes to try (validator nodes are recommended to do pruning anyway).

VersionDB stores multiple versions of on-chain state key-value pairs directly, without using a merklized tree structure like IAVL tree, both db size and query performance are much better than IAVL tree. The major lacking feature compared to IAVL tree is root hash and merkle proof generation, so we still need IAVL tree for those tasks.

Currently grpc query service don't need to support proof generation, so versiondb alone is enough to support grpc query service, there's already a `--grpc-only` flag for one to start a standalone grpc query service.

There could be different implementations for the idea of versiondb, the current implementation we delivered is based on rocksdb v7's experimental user-defined timestamp[^1], it stores the data in a standalone rocksdb instance, it don't support other db backend yet, but the other databases in the node still support multiple backends as before.

After versiondb is enabled, there's no point to keep the full the archived IAVL tree anymore, it's recommended to prune the IAVL tree to keep only recent versions, for example versions within the unbonding period or even less.

## Configuration

To enable versiondb, set the `versiondb.enable` to `true` in `app.toml`:

```toml
[versiondb]
enable = true
```

On startup, the node will create a `StreamingService` to subscribe to latest state changes in realtime and save them to versiondb, the db instance is placed at `$NODE_HOME/data/versiondb` directory, there's no way to customize the db path currently. It'll also switch grpc query service's backing store to versiondb from IAVL tree, you should migrate the legacy states in advance to make the transition smooth, otherwise, the grpc queries can't see the legacy versions.

If the versiondb is not empty and it's latest version doesn't match the IAVL db's last committed version, the startup will fail with error message `"versiondb lastest version %d doesn't match iavl latest version %d"`, that's to avoid creating gaps in versiondb accidentally. When this error happens, you just need to update versiondb to the latest version in iavl tree manually, or restore IAVL db to the same version as versiondb (see [](#catch-up-with-iavl-tree)).

## Migration

Since our chain is pretty big now, a lot of efforts have been put to make sure the transition process can finish in practical time. The migration process will try to parallelize the tasks as much as possible, and use significant ram, but there's flags for user to control the concurrency level and ram usage to make it runnable on different machine specs.

The legacy state migration process is done in two main steps:

- Extract state change sets from existing archived IAVL tree.
- Feed the change set files to versiondb.

### Extract Change Sets

```bash
$ cronosd changeset dump data --home /chain/.cronosd
```

`dump` command will extract the change sets from the IAVL tree, and store each store in separate directories, it use the store list registered in current version of `App` by default, you can customize that with `--stores` parameter. The change set files are segmented into different block chunks and compressed with zlib level 6 by default, the chunk size defaults to 1m blocks, the result `data` directly looks like this:

```
data/acc/block-0.zz
data/acc/block-1000000.zz
data/acc/block-2000000.zz
...
data/authz/block-0.zz
data/authz/block-1000000.zz
data/authz/block-2000000.zz
...
```

The extraction is the slowest step, the test run on testnet archive node takes around 11 hours on a 8core ssd machine, but fortunately, the change set files can be verified pretty fast(a few minutes), so they can be share on CDN in a trustless manner, normal users should just download them from CDN and verify the correctness locally, should be much faster than extract by yourself.

For rocksdb backend, `dump` command opens the db in readonly mode, it can run on live node's db, but goleveldb backend don't support this feature yet.

#### Verify Change Sets

```bash
$ cronosd changeset verify data
35b85a775ff51cbcc48537247eb786f98fc6a178531d48560126e00f545251be
{"version":"189","storeInfos":[{"name":"acc","commitId":{"version":"189" ...
```

`verify` command will replay all the change sets and rebuild the target IAVL tree and output the app hash and commit info of the target version (defaults to latest version in the change sets), then user can manually check the app hash against the block headers.

`verify` command takes several minutes and several gigabytes of ram to run, if ram usage is a problem, it can also run incrementally, you can export the snapshot for a middle version, then verify the remaining versions start from that snapshot:

```bash
$ cronosd changeset verify data --save-snapshot snapshot --target-version 3000000
$ cronosd changeset verify data --load-snapshot snapshot
```

The format of change set files are documented [here](memiavl/README.md#change-set-file).

### Build VersionDB

To maximize the speed of initial data ingestion speed into rocksdb, we take advantage of the sst file writer feature to write out sst files first, then ingest them into final db, the sst files for each store can be written out in parallel. We also developed an external sorting algorithm to sort the data before writing the sst files, so the sst files don't have overlaps and can be ingested into the bottom-most level in final db.

```bash
$ cronosd changeset build-versiondb-sst ./data ./sst
$ cronosd changeset ingest-versiondb-sst /home/.cronosd/data/versiondb sst/*.sst --move-files --maximum-version 189
```

User can control the peak ram usage by controlling the `--concurrency` and `--sorter-chunk-size`.

With default parameters it can finish in around 12minutes for testnet archive node on our test node (8cores, peak RSS 2G).

### Restore IAVL Tree

When migrating an existing archive node to versiondb, it's recommended to rebuild the `application.db` from scratch to reclaim disk space faster, we provide a command to restore a single version of IAVL trees from memiavl snapshot.

```bash
$ # create memiavl snapshot
$ cronosd changeset verify data --save-snapshot snapshot
$ # restore application.db
$ cronosd changeset restore-app-db snapshot application.db
```

Then replace the whole `application.db` in the node with the newly generated one.

It only takes a few minutes to run on our testnet archive node, it only suppot generating rocksdb `application.db` right now, so please set `app-db-backend="rocksdb"` in `app.toml`.

### Catch Up With IAVL Tree

If an non-empty versiondb lags behind from the current `application.db`, the node will refuse to startup, in this case user can either sync versiondb to catch up with  `application.db`, or simply restore the  `application.db` with the correct version of snapshot. To catch up, you can follow the similar procedure as migrating from genesis, just passing the block range in change set dump command.

[^1]: https://github.com/facebook/rocksdb/wiki/User-defined-Timestamp-%28Experimental%29
