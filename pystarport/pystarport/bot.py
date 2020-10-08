import threading
import time

import yaml


class TxJobThread(threading.Thread):
    def __init__(self, label, job, cluster_cli):
        threading.Thread.__init__(self)
        self.label = label
        self.job = job
        self.cluster_cli = cluster_cli

    def transfer_tx_job(self):
        from_address = self.cluster_cli.address(
            self.job["from_account"], self.job.get("node", 0),
        )
        to_address = self.job["to_address"]
        amount = self.job.get("amount", "1basecro")
        interval = self.job.get("interval", 60)

        while 1:
            begin = time.time()
            print(
                "[%s] Transfer %s from %s to %s"
                % (self.label, amount, from_address, to_address)
            )
            result = self.cluster_cli.transfer(from_address, to_address, amount)
            print(result)

            duration = time.time() - begin
            if duration < interval:
                sleep = interval - duration
                print("Next transfer in %ds ...\n" % sleep)
                time.sleep(sleep)

    def run(self):
        # TODO: support more transaction types
        self.transfer_tx_job()


class BotCLI:
    "transacction bot CLI"

    def __init__(self, config_path, cluster_cli):
        self.config = yaml.safe_load(open(config_path))
        self.cluster_cli = cluster_cli

    def start(self):
        """
        prepare and start a transaction bot from configuration
        """
        threads = []
        for i, job in enumerate(self.config["jobs"], start=1):
            thread = TxJobThread(job.get("label", i), job, self.cluster_cli)

            threads.append(thread)
            thread.start()

        for thread in threads:
            thread.wait()
