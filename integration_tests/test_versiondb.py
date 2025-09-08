import shutil
import tempfile
from pathlib import Path

import pytest
import tomlkit
from pystarport.ports import rpc_port

from .utils import cluster_fixture, wait_for_new_blocks, wait_for_port


def cluster(worker_index, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/default.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


def test_versiondb_migration(cluster):
    """
    test versiondb migration commands.
    node0 has memiavl and versiondb enabled while node1 don't,
    - stop all the nodes
    - dump change set from node1's application.db
    - verify change set and save snapshot
    - restore pruned application.db from the snapshot
    - replace node1's application.db with the restored one
    - rebuild versiondb for node0
    - start the nodes, now check:
      - the network can grow
      - node0 do support historical queries
      - node1 don't support historical queries
    """
    community_addr = cluster.address("community")
    reserve_addr = cluster.address("reserve")

    block0 = cluster.block_height()
    cm_balance0 = cluster.balance(community_addr)
    rs_balance0 = cluster.balance(reserve_addr)

    cluster.transfer(community_addr, reserve_addr, "1cro")

    block1 = cluster.block_height()
    cm_balance1 = cluster.balance(community_addr)
    rs_balance1 = cluster.balance(reserve_addr)

    assert cm_balance1 == cm_balance0 - 100000000
    assert rs_balance1 == rs_balance0 + 100000000

    # wait for a few blocks
    wait_for_new_blocks(cluster, 2)

    # stop the network first
    print("stop all nodes")
    cluster.supervisor.stopAllProcesses()

    # check the state of all nodes should be stopped
    for info in cluster.supervisor.getAllProcessInfo():
        assert info["statename"] == "STOPPED"

    node0 = cluster.cosmos_cli(i=0)
    node1 = cluster.cosmos_cli(i=1)
    # dump change set from node1's application.db
    changeset_dir = tempfile.mkdtemp(dir=cluster.data_root)
    print("dump to:", changeset_dir)
    # only restore to an intermidiate version to test version mismatch behavior
    print(node1.changeset_dump(changeset_dir, end_version=block1 + 1))

    # verify and save to snapshot
    snapshot_dir = tempfile.mkdtemp(dir=cluster.data_root)
    print("verify and save to snapshot:", snapshot_dir)
    _, commit_info = node0.changeset_verify(changeset_dir, save_snapshot=snapshot_dir)
    latest_version = commit_info["version"]

    # replace existing `application.db`
    app_db1 = node1.data_dir / "data/application.db"
    print("replace node db:", app_db1)
    shutil.rmtree(app_db1)
    print(node1.changeset_restore_app_db(snapshot_dir, app_db1))

    print("restore versiondb for node0")
    sst_dir = tempfile.mkdtemp(dir=cluster.data_root)
    print(node0.changeset_build_versiondb_sst(changeset_dir, sst_dir))

    # ingest-versiondb-sst expects an empty database
    versiondb_path = node0.data_dir / "data/versiondb"
    if versiondb_path.exists():
        shutil.rmtree(node0.data_dir / "data/versiondb")
    print(
        node0.changeset_ingest_versiondb_sst(
            node0.data_dir / "data/versiondb", sst_dir, maximum_version=latest_version
        )
    )

    # force node1's app-db-backend to be rocksdb
    patch_app_db_backend(node1.data_dir / "config/app.toml", "rocksdb")

    print("start all nodes")
    cluster.supervisor.startProcess(f"{cluster.chain_id}-node{0}")
    cluster.supervisor.startProcess(f"{cluster.chain_id}-node{1}")
    wait_for_port(rpc_port(cluster.base_port(0)))
    wait_for_port(rpc_port(cluster.base_port(1)))

    # node0 supports historical queries with versiondb
    assert node0.balance(community_addr, height=block0) == cm_balance0
    assert node0.balance(community_addr, height=block1) == cm_balance1
    assert cluster.balance(community_addr) == cm_balance1

    # check query still works, node1 don't enable versiondb,
    # so we are testing iavl query here.
    assert node1.balance(community_addr) == cm_balance1

    # Verify node1 cannot support historical queries
    with pytest.raises(Exception):
        # This should fail because node1 doesn't have versiondb enabled
        node1.balance(community_addr, height=block0)

    # check the chain is still growing
    cluster.transfer(community_addr, reserve_addr, "1cro")

    cm_balance2 = cluster.balance(community_addr)
    rs_balance2 = cluster.balance(reserve_addr)

    assert cm_balance2 == cm_balance1 - 100000000
    assert rs_balance2 == rs_balance1 + 100000000


def patch_app_db_backend(path, backend):
    cfg = tomlkit.parse(path.read_text())
    cfg["app-db-backend"] = backend
    path.write_text(tomlkit.dumps(cfg))
