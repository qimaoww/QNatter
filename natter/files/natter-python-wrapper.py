#!/usr/bin/env python3

import runpy
import sys


NATTER_SCRIPT = "/usr/bin/natter.py"
PR_SET_NAME = 15


def set_process_name(name):
    encoded_name = name.encode("ascii")[:15]
    try:
        with open("/proc/self/comm", "wb") as proc_comm:
            proc_comm.write(encoded_name + b"\n")
    except OSError:
        pass

    try:
        import ctypes

        libc = ctypes.CDLL(None)
        libc.prctl(PR_SET_NAME, ctypes.c_char_p(encoded_name), 0, 0, 0)
    except Exception:
        pass


def main():
    set_process_name("Natter")
    sys.argv[0] = NATTER_SCRIPT
    runpy.run_path(NATTER_SCRIPT, run_name="__main__")


if __name__ == "__main__":
    main()
