import logging

import sys
from os.path import dirname, basename

BASE_NAME = basename(dirname(__file__))
logging.getLogger("urllib3").setLevel(logging.ERROR)
logging.getLogger("requests").setLevel(logging.ERROR)
logging.basicConfig(
    filename="%s.log" % BASE_NAME,
    level=logging.INFO,
    format="%(asctime)s - %(message)s",
)
console = logging.StreamHandler(sys.stdout)
formatter = logging.Formatter("%(asctime)s - %(message)s")
console.setFormatter(formatter)

logger = logging.getLogger(BASE_NAME)
logger.addHandler(console)