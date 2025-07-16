import pytest
from pystarport import cluster


@pytest.fixture(scope="module")
def maxsupply_cluster(tmp_path_factory):
    """Create a test cluster with maxsupply module enabled"""
    path = tmp_path_factory.mktemp("maxsupply")
    
    # Custom configuration for maxsupply testing
    config = {
        "app-config": {
            "maxsupply": {
                "max_supply": "100000000000000000000000000"  # 100M CRO
            }
        }
    }
    
    with cluster.Cluster(
        path,
        "maxsupply-test",
        config=config
    ) as cluster_instance:
        yield cluster_instance