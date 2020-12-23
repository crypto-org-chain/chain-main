import random
import sys
import threading
import time

import yaml

from .cluster import ClusterCLI
from .cosmoscli import CosmosCLI


class TxJobThread(threading.Thread):
    def __init__(self, label, job, cosmos_cli: CosmosCLI):
        threading.Thread.__init__(self)
        self.label = label
        self.job = job
        self.cosmos_cli = cosmos_cli

    def transfer_tx_job(self):
        from_address = self.cosmos_cli.address(
            self.job["from_account"],
        )
        to_address = self.job["to_address"]
        if "random_amount" in self.job:
            amount = random_amount(
                self.job["random_amount"][0],
                self.job["random_amount"][1],
                self.job["random_amount"][2],
            )
        else:
            amount = self.job["amount"]

        print(
            "[%s] Transfer %s from %s to %s"
            % (self.label, amount, from_address, to_address)
        )
        result = self.cosmos_cli.transfer(from_address, to_address, amount)
        print(result)

    def delegate_tx_job(self):
        from_address = self.cosmos_cli.address(self.job["from_account"])
        to_address = self.job["to_validator_address"]
        if "random_amount" in self.job:
            amount = random_amount(
                self.job["random_amount"][0],
                self.job["random_amount"][1],
                self.job["random_amount"][2],
            )
        else:
            amount = self.job["amount"]

        print(
            "[%s] Delegate %s from %s to %s"
            % (self.label, amount, from_address, to_address)
        )
        result = self.cosmos_cli.delegate_amount(to_address, amount, from_address)
        print(result)

    def withdraw_all_rewards_job(self):
        from_address = self.cosmos_cli.address(self.job["from_account"])
        print("[%s] Withdraw all rewards from %s" % (self.label, from_address))
        result = self.cosmos_cli.withdraw_all_rewards(from_address)
        print(result)

    def next_interval(self):
        if "random_interval" in self.job:
            return random.randint(
                self.job["random_interval"][0],
                self.job["random_interval"][1],
            )
        return self.job["interval"]

    def run(self):
        job_type = self.job["type"]
        while 1:
            begin = time.time()

            try:
                if job_type == "transfer":
                    self.transfer_tx_job()
                elif job_type == "delegate":
                    self.delegate_tx_job()
                elif job_type == "withdraw_all_rewards":
                    self.withdraw_all_rewards_job()
                else:
                    print("Unknown job type: %s", job_type)
                    sys.exit()
            except Exception as e:
                print("error executing job:", sys.exc_info(), str(e))

            interval = self.next_interval()

            duration = time.time() - begin
            if duration < interval:
                sleep = interval - duration
                print("Next %s in %ds ...\n" % (job_type, sleep))
                time.sleep(sleep)


def random_amount(min, max, denom):
    return "%d%s" % (random.randint(min, max), denom)


class BotClusterCLI:
    "transaction bot Cluster CLI"

    def __init__(self, config_path, cluster_cli: ClusterCLI):
        self.config = yaml.safe_load(open(config_path))
        self.cluster_cli = cluster_cli

    def start(self):
        """
        prepare and start a transaction bot from configuration
        """
        threads = []
        for i, job in enumerate(self.config["jobs"], start=1):
            node_i = job.get("node", 0)
            cli = CosmosCLI(
                self.cluster_cli.home(node_i),
                self.cluster_cli.node_rpc(node_i),
                chain_id=self.cluster_cli.chain_id,
                cmd=self.cluster_cli.cmd,
            )
            thread = TxJobThread(job.get("label", i), job, cli)

            threads.append(thread)
            thread.start()

        for thread in threads:
            thread.join()


class BotCLI:
    "transaction bot CLI"

    def __init__(self, config_path, cosmos_cli=None):
        self.config = yaml.safe_load(open(config_path))
        self.cosmos_cli = cosmos_cli

    def start(self):
        """
        prepare and start a transaction bot from configuration
        """
        threads = []
        for i, job in enumerate(self.config["jobs"], start=1):
            thread = TxJobThread(job.get("label", i), job, self.cosmos_cli)

            threads.append(thread)
            thread.start()

        for thread in threads:
            thread.join()
