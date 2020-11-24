import requests


class ApiUtil:
    def __init__(self, ip_port):
        self.base_url = f"http://127.0.0.1:{ip_port}"

    def balance(self, address):
        url = f"{self.base_url}/cosmos/bank/v1beta1/balances/{address}"
        response = requests.get(url)
        balance = response.json()["balances"]
        if len(balance) > 0:
            return int(balance[0]["amount"])
        else:
            return 0

    def account_info(self, address):
        url = f"{self.base_url}/cosmos/auth/v1beta1/accounts/{address}"
        response = requests.get(url)
        account_info = response.json()["account"]
        account_num = int(account_info["account_number"])
        sequence = int(account_info["sequence"])
        return {"account_num": account_num, "sequence": sequence}

    def broadcast_tx(self, signed_tx: dict):
        url = f"{self.base_url}/txs"
        response = requests.post(url, json=signed_tx)
        if not response.ok:
            raise Exception(
                f"response code: {response.status_code}, {response.reason}, {response.json()}"
            )
        result = response.json()
        if result.get("code"):
            raise Exception(result["raw_log"])
        return result
