import yaml
import threading
import time


class txJobThread(threading.Thread):
    def __init__(self, label, account, cluster_cli):
        threading.Thread.__init__(self)
        self.label = label
        self.account = account
        self.cluster_cli = cluster_cli

    def transfer_tx_job(self):
        amount = self.account.get("amount", "1basecro")
        interval = self.account.get("interval", 60)
        while 1:
            begin = time.time()
            print("Transfer %s from account %s" % (amount, self.label))
            result = self.cluster_cli.transfer(
                self.account["from"], self.account["to"], amount,
            )
            print(result)
            duration = time.time() - begin
            if duration < interval:
                sleep = interval - duration
                print("Next transfer happens in %ds ...\n" % sleep)
                time.sleep(sleep)

    def run(self):
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
        for i, account in enumerate(self.config.get("accounts"), start=1):
            thread = txJobThread(account.get("name", i), account, self.cluster_cli)
            thread.start()
            thread.join()
