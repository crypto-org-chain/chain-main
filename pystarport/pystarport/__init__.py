import os
import sys
from pathlib import Path

proto_folder = Path(os.path.abspath(__file__)).parent.joinpath("proto_python")
sys.path.append(str(proto_folder))
